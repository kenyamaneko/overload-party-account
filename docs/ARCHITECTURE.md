# Account サービス設計

本ドキュメントは **コードを読んでも一見しては分からない設計意図** だけを残す。実装詳細（エンドポイントごとのバリデーション順・SQL 文・エラー → HTTP ステータス変換・環境変数の一覧）は各ファイルの実装とコメントを一次情報とする。

サービス概要・起動手順は [../README.md](../README.md)、エンドポイントは [API_REFERENCE.md](API_REFERENCE.md)（自動生成）、DB スキーマは [DATA_DESIGN.md](DATA_DESIGN.md)、ビジネス仕様は [FEATURE_SPEC.md](FEATURE_SPEC.md) を参照。

## 1. account サービスの責務境界

account は **プレイヤーの素性（players）と周辺属性（設定・ファクション所有・経験値・デイリーバトル）** の single source of truth。他サービスが必要とする情報は REST 経由でのみ公開する。

### 1.1 スキーマ所有 (SSoT と read model)

| ドメイン | SSoT | account 側の扱い |
|---|---|---|
| プレイヤー本体 | `account.players` (account) | authoritative |
| ユーザー設定 | `account.user_settings` (account) | authoritative |
| デイリーバトル回数 | `account.player_daily_battle` (account) | authoritative |
| ファクション所有 | `account.player_factions` (account) | authoritative。shop が `shop.player_owned_factions` で同一状態の read model を持つ |
| プレミアム状態 | `shop.subscriptions` (shop) | `players.is_premium` / `premium_expires_at` は account 側の射影 |
| ゲームバランス定数 | Cloud Firestore `game_config` | read-only。起動時・リクエスト時に参照 |
| ファクションマスター | `common/data/factions.yaml` | code-generate された定数を参照 |

プレミアム状態の authoritative は shop 側のサブスクリプション契約。account は `premium-updated` イベントの subscriber として **最終的整合 (eventually consistent) な射影** を保持する。`is_premium` を参照するほぼ全ての REST レスポンスでローカル SELECT の安定性が必要なため、JOIN ではなく射影として持っている。

### 1.2 他サービス呼び出しの禁則

account は他サービスを直接呼び出さない（gateway / shop / scenario / card / battle 含めて）。

- 状態の取り込みは **Pub/Sub subscribe で片方向**
- account から外部への副作用は **REST レスポンスを返すのみ**（Pub/Sub publish しない）

この非対称性は ADR-011 / ADR-014 の schema ownership 契約から来ている。account は「呼ばれるだけ」のサービスに徹するため、外部 API 呼び出しの retry / circuit breaker が実装コードに登場しない。

## 2. 認証信頼境界

account は Firebase Auth の ID Token を検証しない。**認証は gateway で完結している前提** で、gateway が検証済みの `firebase_uid` または解決済みの `playerId` をリクエストボディ / パスパラメータとして forward する。

この信頼境界は 3 つの構造で支えている:

1. **ネットワーク**: account は ClusterIP Service のみで公開し、Ingress に乗らない。外部からの到達経路は gateway 経由のみ
2. **ルーター**: [internal/router/router.go](../internal/router/router.go) は `/internal/v1/...` をベースに全エンドポイントを生やし、認証ミドルウェアを一切挟まない
3. **契約**: `/auth/register` / `/auth/login` は gateway が ID Token を検証して抽出した `firebase_uid` を受け取る。account 側では文字列として扱うだけ

新しいエンドポイントを追加するときも、認証ミドルウェアを account 側に導入しないこと。導入した瞬間に「account の中で認証する責務」が混入し、gateway との二重管理になる。

## 3. Register フローのトランザクション設計

`POST /internal/v1/auth/register` は 3 テーブルを同一トランザクションで初期化する:

```
BEGIN TX
  INSERT INTO account.players             (新規 UUID)
  INSERT INTO account.player_daily_battle (player_id, count=0, last_reset_date=today)
  INSERT INTO account.user_settings       (player_id, デフォルト値)
COMMIT
```

### 3.1 なぜスターターカードや初期ファクションを含めないか

かつて `auth_service.Register` でスターターカード配布と初期ファクション選択を同時に行っていたが、以下の理由で削除済み:

- ファクション選択は UX 上「チュートリアル開始直後の選択画面」という非同期な操作で、登録と同時に決められない
- カード配布は card サービスの責務で、account が card の内部状態を知るべきでない

現在のトリガーポイントは **scenario サービスの「初期ファクション選択完了」イベント** で、account は `faction-selected` イベントを受けて `player_factions` に追加するだけ（§5.1）。`auth_service.Register` にカード付与・ファクション付与を再導入してはいけない（CLAUDE.md 禁止事項）。

### 3.2 重複登録は 409 で吸収

`firebase_uid` は UNIQUE INDEX `idx_players_firebase_uid` で守られている。二重登録は `FindByFirebaseUID` による pre-check と UNIQUE 制約の両方で弾き、handler は `ErrPlayerAlreadyRegistered` を 409 に変換する。Register は冪等ではない（重複は 409 で返す）。gateway 側で冪等リトライしたい場合は Login にフォールバックする契約になっている。

## 4. デイリーバトル制限

デイリーバトル制限は `account.player_daily_battle` テーブル（players と 1:1）で管理する。

### 4.1 ゲーム日の境界 (JST 05:00)

ゲーム日は **JST 05:00** にリセットする。UTC に +4 時間のオフセットを加算して日付部分を取り出す:

| 実時刻 | ゲーム日 |
|---|---|
| JST 2024-01-02 04:59 (UTC 2024-01-01 19:59) | 2024-01-01 |
| JST 2024-01-02 05:00 (UTC 2024-01-01 20:00) | 2024-01-02 |

実装: `service.gameDay()` が `time.Now().UTC().Add(4h)` の日付部分を返す。`gameDayOffset` 定数で一箇所に閉じている ([internal/service/player_service.go](../internal/service/player_service.go))。

### 4.2 リセットと上限判定

`GetBattleLimit` と `IncrementBattleCount` の両方で以下を行う:

1. 現在のゲーム日を算出
2. `player_daily_battle.last_reset_date` が当日と異なるなら論理的にリセット済みとみなす（カウント 0）
3. 上限は Firestore `game_config.free_daily_battle_limit` から読む
4. プレミアム会員は上限判定をスキップ（`GetBattleLimit` では `DailyBattleLimit = -1` / `CanBattle = true` を返す）

`free_daily_battle_limit` が未設定（値 0）のときは `GetBattleLimit` がエラーを返す（フォールバック禁止）。Firestore 上でキーが存在しない状態は運用事故として扱う。

### 4.3 IncrementBattleCount の冪等性と並行制御

IncrementBattleCount は **プレミアム会員でもカウントを記録する**。上限は `GetBattleLimit` 側で判定する構造で、記録と判定を分離している（battle サービスから毎試合呼ばれる純粋な副作用）。

同一プレイヤーに対する並行 increment のシリアライズは `pg_player_repo.IncrementDailyBattle` 内の UPDATE の row lock に任せている。`player_daily_battle` は PK 単一行のため、デッドロックリスクは低い。

## 5. Pub/Sub subscriber

account は 2 つの Pub/Sub subscription を常駐 worker として pull する。両方とも Exactly-Once Delivery を前提にしつつ、**アプリ層で `processed_events` による冪等ガード** を併用する二層防御。

### 5.1 faction-selected subscriber

subscription: `faction-selected-account-sub` (`FACTION_SELECTED_SUBSCRIPTION` で上書き可)

発行元と `source`:

| publisher | source | 副作用 |
|---|---|---|
| scenario | `scenario_initial` | `player_factions` INSERT + `players.selected_faction` UPDATE |
| shop | `shop_purchase` | `player_factions` INSERT のみ（selected_faction は変更しない） |

処理（[internal/adapter/pubsub/faction_selected_subscriber.go](../internal/adapter/pubsub/faction_selected_subscriber.go)）:

```
BEGIN TX
  INSERT processed_events (event_id, event_type)  ← ON CONFLICT DO NOTHING
  IF 既存行だった: COMMIT; ACK; return
  INSERT player_factions (player_id, faction, source)
  IF source == scenario_initial: UPDATE players SET selected_faction = ...
COMMIT → ACK
```

`source == shop_purchase` で `selected_faction` を更新しないのは、ショップ購入は「ロスター追加」であり、プレイヤーが能動的に切り替える前に勝手にアクティブ選択を上書きしてはいけないため。アクティブ切り替えは `PUT /players/:id/faction` の独立したエンドポイント。

### 5.2 premium-updated subscriber

subscription: `premium-updated-account-sub` (`PREMIUM_UPDATED_SUBSCRIPTION` で上書き可)

発行元: shop のみ。shop がサブスクリプション状態遷移（開始 / 更新 / 期限切れ / 失効）のうち **premium が変化する遷移で publish する**（cancel 時は publish しない契約、shop 側 ARCHITECTURE.md 参照）。

処理:

```
BEGIN TX
  INSERT processed_events ...  ← 冪等ガード
  UPDATE players SET is_premium=$, premium_expires_at=$ WHERE player_id=$
COMMIT → ACK
```

### 5.3 `processed_events` による冪等性契約

- 1 行 = (`event_id`, `event_type`, `processed_at`) の 3 カラム。`event_id` が PK
- subscriber はトランザクション冒頭で `INSERT ... ON CONFLICT DO NOTHING RETURNING event_id`
- `RETURNING` が空 → 既に処理済み → トランザクション内の後続処理をスキップして ACK
- 処理本体と `processed_events` INSERT を同一トランザクションに揃えているため、部分適用は発生しない

Pub/Sub の Exactly-Once 配信がインフラ層の第一防御。`processed_events` はアプリ層の第二防御で、Exactly-Once 契約が破れた場合（再配信・観測ウィンドウ外のリトライ・メッセージの手動 replay）のセーフティネット。

### 5.4 エラー時の NACK と DLQ

| 失敗種別 | 動作 |
|---|---|
| JSON デシリアライズ失敗 | NACK（Pub/Sub 側で再配信 → 最終的に DLQ へ） |
| `event_type` が未知 | ACK（握りつぶしではなく「この subscriber の責務外」として意図的にスキップ。ログは残す） |
| DB エラー / トランザクション失敗 | NACK（一時的障害としてリトライさせる） |

未知の `event_type` を ACK するのは、将来 publisher 側で新しいイベント種別を追加した際に account の subscriber を止めないため。既知の event_type のペイロードが壊れているケースは JSON デシリアライズ失敗側に分岐する。

## 6. 経験値・レベル計算

### 6.1 係数の SSoT は Firestore

経験値獲得量 (`exp_win` / `exp_loss` / `exp_draw`) とレベル計算係数 (`exp_formula_coefficient`) は Firestore `game_config` から読む。DB には置かない。

理由: ゲームバランス調整は運用者が Firestore コンソールからコードデプロイなしで変えるため。account 側はハードコードしない。

未設定（値 0 または存在しない）は起動エラーにせず、当該リクエストをエラーにする（運用者が値を戻すまで battle 側で経験値が積めない）。

### 6.2 レベル上限の「増加のみ」契約

`ComputeLevel` は現在レベルからの **増加のみ** 計算する。

> 「係数を厳しく変えた後に経験値を追加したら既存プレイヤーのレベルが下がる」事故を避けるため、`newExp < nextLevelExp` のループで増加方向にしかレベルを進めない。

係数変更の遡及はしない。運用上「プレイヤーはレベルが下がらない」という UX 契約を守るための実装上の選択。

### 6.3 AwardGameExp の NPC 扱い

`AwardGameExp(player1, player2, winnerNum, reason, matchType)` は battle 終了時に両プレイヤーへ同時に exp を配る唯一の入口。

- `matchType == "npc"` のとき player2 側を NPC とみなし exp 付与をスキップ
- `reason == "draw"` または `winnerNum == 0` は両者 `exp_draw`
- それ以外は勝者 `exp_win` / 敗者 `exp_loss`

`matchType` / `reason` / `winnerNum` の定義は `overload-party-common` と `overload-party-battle` の共有定数パッケージが SSoT（リテラル禁止）。

## 7. エラーハンドリング

### 7.1 レイヤ別の責務

| 層 | 返す/扱う |
|---|---|
| repository | `port.ErrNotFound` と wrap された SQL エラー |
| service | ドメインのセンチネル（`ErrPlayerNotFound` / `ErrPlayerAlreadyRegistered` / `ErrInvalidFaction` / `ErrFactionAlreadySelected`）+ wrap された下位エラー |
| handler (rest) | `errors.Is` でセンチネルを分類し HTTP ステータスに変換（[internal/handler/rest/errors.go](../internal/handler/rest/errors.go)） |

service 層は HTTP ステータスを知らず、handler 層は SQL を知らない。

### 7.2 センチネル → HTTP ステータス

| センチネル | HTTP | 意味 |
|---|---|---|
| `port.ErrNotFound` / `service.ErrPlayerNotFound` | 404 | 対象プレイヤーが存在しない |
| `service.ErrPlayerAlreadyRegistered` | 409 | 同一 firebase_uid で登録済み |
| `service.ErrFactionAlreadySelected` | 409 | 初期ファクション選択済み（冪等な「成功扱い」のためクライアントはエラーとして表示しない） |
| `service.ErrInvalidFaction` | 400 | ファクション値が `gamedesign.SelectableFactions` に含まれない |
| その他 | 500 | wrap された下位エラー（DB 接続断等） |

### 7.3 握りつぶし禁止

DB エラー・Pub/Sub デシリアライズ失敗・Firestore 読み取り失敗をログのみで握りつぶさない（CLAUDE.md 設計思想）。

例外: 未知の `event_type` の ACK は「意図的な no-op」で、握りつぶしではない（§5.4）。

## 8. 運用

### 8.1 環境変数 / Secret Manager

環境変数の一覧と必須条件は [internal/config/config.go](../internal/config/config.go) の `FromEnv` が SSoT（欠ければ即 fail）。運用上の注意点のみ:

- `DATABASE_URL` は Secret Manager 由来。k8s マニフェストにインラインしない
- `PUBSUB_PROJECT_ID` / `FIRESTORE_PROJECT_ID` は ConfigMap 経由で環境ごとに切り替え
- `FACTION_SELECTED_SUBSCRIPTION` / `PREMIUM_UPDATED_SUBSCRIPTION` はデフォルトで本番名と一致するため通常は未設定でよい。環境分離検証時のみ上書き

### 8.2 Pub/Sub トピックと subscriber

| トピック | 発行元 | account の subscription | account 側の副作用 |
|---|---|---|---|
| `faction-selected` | scenario / shop | `faction-selected-account-sub` | `player_factions` INSERT / `selected_faction` UPDATE (§5.1) |
| `premium-updated` | shop | `premium-updated-account-sub` | `players.is_premium` / `premium_expires_at` UPDATE (§5.2) |

account 自身はトピックを publish しない。

### 8.3 Firestore の運用

`game_config` コレクションは運用者が手動で値を書く（コード上には生成スクリプトを持たない）。キーのリストと意味は [FEATURE_SPEC.md](FEATURE_SPEC.md) と `service/player_service.go` の定数を参照。
