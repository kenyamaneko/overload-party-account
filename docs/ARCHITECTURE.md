# Account サービス設計

このドキュメントは account サービスの内部動作を説明する。サービスの概要・エンドポイント・環境変数は [README.md](../README.md) を参照。

## Firebase Auth 連携フロー

Account サービス自体は Firebase Auth のトークン検証を行わない。Gateway が Firebase ID Token を検証し、`firebaseUID` または解決済みの `playerId` を forward する。

### 登録フロー

1. クライアント -> Gateway: Firebase ID Token 付きリクエスト
2. Gateway: トークン検証 -> `firebaseUID` 抽出
3. Gateway -> Account: `POST /internal/v1/auth/register` with `{firebase_uid, username}`
4. Account: 単一トランザクションで `players` + `user_settings` (デフォルト値) + `player_daily_battle` 行を作成
5. 409 Conflict: `firebase_uid` UNIQUE 制約による重複検出 (冪等)

### ログインフロー

1. Gateway -> Account: `POST /internal/v1/auth/login` with `{firebase_uid}`
2. Account: `players` から `firebase_uid` で検索、見つからなければ 404
3. Gateway: 以降のリクエストで `playerId` をパスパラメータに使用

## Daily battle limit リセットロジック

デイリーバトル制限は `player_daily_battle` テーブルで管理する。

### ゲーム日の定義

JST 05:00 (= UTC 20:00) をゲーム日の境界とする。UTC 時刻に +4h のオフセットを加算して日付部分を取り出す:

- JST 2024-01-02 04:59 -> ゲーム日 2024-01-01
- JST 2024-01-02 05:00 -> ゲーム日 2024-01-02

### IncrementBattleCount

```
SELECT battle_date, battle_count
  FROM account.player_daily_battle
 WHERE player_id = $1
   FOR UPDATE
```

1. `FOR UPDATE` で行ロックを取得 (同一プレイヤーの並行インクリメントを排他)
2. `battle_date` が当日のゲーム日と異なる場合、カウンターを 0 にリセットして日付を更新
3. `battle_count` が上限 (Firestore `game_config` の `free_daily_battle_limit`) に達している場合:
   - `players.is_premium = true` ならプレミアム会員は制限なし
   - それ以外は `ErrBattleLimitReached` を返す
4. `battle_count` をインクリメントして COMMIT

`FOR UPDATE` row lock はデッドロックリスクを避けるために単一行ロックに限定している。player_daily_battle は players と 1:1 で、同一プレイヤーに対する並行リクエストだけがシリアライズされる。

## 経験値・レベル計算

### AwardGameExp

バトル終了時に battle サービスから呼ばれる。両プレイヤーの経験値を 1 リクエストで更新する:

- 勝者: Firestore `game_config` の `exp_formula_coefficient` に基づく base exp
- 敗者: base exp の半分
- `match_type` (pvp / npc) による補正あり

レベルアップ判定は `AddExp` で行い、`game-design-constants` の閾値テーブルに基づいて `players.level` を更新する。

## Pub/Sub subscriber

Account は 2 つの Pub/Sub subscription を pull する。両方とも Exactly-Once Delivery。

### faction-selected subscriber

subscription: `faction-selected-account-sub` (デフォルト)

イベント発行元:
- scenario: 初回ファクション選択時 (source = `scenario_initial`)
- shop: ファクションセット購入時 (source = `shop_purchase`)

処理 (単一トランザクション):
1. `processed_events` に `event_id` を INSERT (冪等性ゲート)
2. 既に処理済みなら no-op で ACK
3. `player_factions` に INSERT (`player_id, faction, source`)
4. `source = scenario_initial` の場合のみ `players.selected_faction` を UPDATE (ショップ購入ではアクティブ選択を変更しない)
5. COMMIT -> ACK

### premium-updated subscriber

subscription: `premium-updated-account-sub` (デフォルト)

イベント発行元: shop (サブスクリプション状態変化時)

処理 (単一トランザクション):
1. `processed_events` に `event_id` を INSERT (冪等性ゲート)
2. 既に処理済みなら no-op で ACK
3. `players.is_premium` / `players.premium_expires_at` を UPDATE
4. COMMIT -> ACK

### processed_events 冪等性

両 subscriber 共通のパターン:

- `account.processed_events (event_id, event_type)` テーブルを使用
- `INSERT ... ON CONFLICT DO NOTHING RETURNING event_id` で重複を検出
- RETURNING が空なら既に処理済み -> トランザクション内の後続処理をスキップ
- 処理本体と `processed_events` INSERT が同一トランザクション内にあるため、部分的な適用は発生しない

Pub/Sub の Exactly-Once Delivery はインフラ層の第一防御。`processed_events` はアプリ層の第二防御で、Exactly-Once が破れた場合のセーフティネット。

## エラーハンドリング

- `DATABASE_URL` / `PUBSUB_PROJECT_ID` 未設定: 起動拒否 (fail-fast)
- Pub/Sub subscription が存在しない: 起動拒否
- subscriber 内の処理失敗: NACK -> Pub/Sub がリトライ
- subscriber 内のペイロードデシリアライズ失敗: NACK (不正メッセージは DLQ へ)
- DB 接続失敗: 各ハンドラーが 500 を返す (フォールバック値で継続しない)
