# Account 機能仕様書

このドキュメントは account サービスがビジネス要件として **何を保証するか** を記述する。実装方法ではなく振る舞いの契約を定義するため、テストは本書の観点で書く。

関連ドキュメント:
- 内部動作・配線・運用設定: [ARCHITECTURE.md](ARCHITECTURE.md)
- HTTP エンドポイント契約（自動生成）: [API_REFERENCE.md](API_REFERENCE.md)
- DB スキーマ: [DATA_DESIGN.md](DATA_DESIGN.md)

---

## 1. サービス責務

account は以下の機能ドメインを所有する。

| 機能 | 主要な責務 |
|---|---|
| プレイヤー登録・ログイン | Firebase UID と player_id の紐付け。初期行の作成（players / player_daily_battle / user_settings） |
| プレイヤー情報の参照・更新 | プレイヤー名・選択ファクション・装備アイコンの参照/更新。レベル進捗の算出 |
| デイリーバトル制限 | JST 05:00 境界の制限回数管理。increment は冪等 |
| ファクション所有 | 初期選択時の `player_factions` 挿入。`faction-selected` イベントからの同期 |
| 経験値・レベル | `AwardGameExp` による両プレイヤー同時付与。係数変更時もレベルは下がらない |
| プレミアムステータス | `premium-updated` イベントから `is_premium` を射影保持 |
| ユーザー設定 | 言語・音量・通知フラグの参照/更新 |

account は **account スキーマの DB 行と Pub/Sub から取り込んだ状態を唯一の真実とし**、他サービスを直接呼び出さず、自らイベントを publish もしない。

---

## 2. Register / Login

### 2.1 `POST /internal/v1/auth/register`

**入力**: `{firebase_uid, username}`（gateway が ID Token 検証済み前提）
**成功**: `201 Created` + `Player` レスポンス
**処理**:

1. `username` が 1〜50 文字（utf-8 rune 数）であることを確認。範囲外は 400
2. `firebase_uid` で既存プレイヤーを pre-check。存在すれば 409 (`ErrPlayerAlreadyRegistered`)
3. 新 UUID を採番し、単一トランザクションで:
   - `players` INSERT (level=1, exp=0, is_premium=false, selected_faction=NULL)
   - `player_daily_battle` INSERT (count=0, last_reset_date=今日の UTC 日付)
   - `user_settings` INSERT (アプリ層デフォルト値)

### 2.2 Register の冪等性契約

- **冪等ではない**。同一 `firebase_uid` の二重 Register は 409 を返す
- gateway 側で「Register を試して 409 なら Login にフォールバック」する契約を採用している。account 単体では吸収しない
- スターターカード配布・初期ファクション選択は Register に含まれない（[ARCHITECTURE.md §3.1](ARCHITECTURE.md)）

### 2.3 `POST /internal/v1/auth/login`

**入力**: `{firebase_uid}`
**成功**: `200 OK` + `Player` レスポンス
**処理**: `players` を `firebase_uid` で検索。不在なら 404 (`ErrPlayerNotFound`)

副作用なし。

---

## 3. プレイヤー情報の参照・更新

| メソッド | パス | 概要 |
|---|---|---|
| GET | `/internal/v1/players/:playerId` | プレイヤー情報 + レベル進捗を返す |
| GET | `/internal/v1/players/by-firebase-uid/:firebaseUID` | Firebase UID からプレイヤーを引く |
| PUT | `/internal/v1/players/:playerId/name` | username 更新 |
| PUT | `/internal/v1/players/:playerId/faction` | アクティブ選択ファクション更新 |
| PUT | `/internal/v1/players/:playerId/premium` | プレミアムステータス更新（battle 等の内部呼び出し用） |
| POST | `/internal/v1/players/:playerId/factions` | ファクションの明示的付与（運用用） |
| GET | `/internal/v1/players/:playerId/factions` | 所持ファクション一覧 |

### 3.1 `GetPlayerResponse` のレベル進捗

GetPlayer レスポンスには現在レベル内の `level_exp_current` と `level_exp_required` を含める。

- `level_exp_current = max(0, exp - coeff * level * level)`
- `level_exp_required = coeff * (level+1) * (level+1) - coeff * level * level`
- `coeff` は Firestore `game_config.exp_formula_coefficient`。未設定（0 以下）は 500 エラー

---

## 4. デイリーバトル制限

### 4.1 ゲーム日の境界

ゲーム日は **JST 05:00 = UTC 20:00** にリセットする。実装は `time.Now().UTC().Add(4h)` の日付部分を取る（[ARCHITECTURE.md §4.1](ARCHITECTURE.md)）。

### 4.2 `GET /internal/v1/players/:playerId/battle-limit`

`BattleLimitResponse` を返す:

| フィールド | 意味 |
|---|---|
| `daily_battle_count` | 現在のカウンタ値（`last_reset_date` がゲーム日と異なれば 0 扱い） |
| `daily_battle_limit` | free の上限。プレミアム時は `-1`（無制限シグナル） |
| `can_battle` | `daily_battle_count < daily_battle_limit`。プレミアム時は常に true |

上限値は Firestore `game_config.free_daily_battle_limit`。値 0（未設定）は 500 エラー。

### 4.3 `POST /internal/v1/players/:playerId/battle-limit/increment`

battle サービスが試合開始時に呼ぶ。

- **プレミアム会員でもカウントを記録する**（上限判定は行わない）
- 当日のゲーム日と `last_reset_date` が異なれば 0 にリセットしてから 1 に
- レスポンスは 204 No Content

冪等ではない（呼ぶたびにインクリメント）。同一試合に対する二重呼び出しを避けるのは battle 側の責務。

---

## 5. ファクション所有

### 5.1 初期ファクション選択 (`POST /internal/v1/players/:playerId/factions/select`)

**入力**: `{faction_id, source?}`
**成功**: `200 OK`

バリデーション:

1. `playerId` が空 → 400
2. `faction_id` が空 → 400
3. `source` 指定時は `"initial_selection"` 以外 → 400
4. `faction_id` が `gamedesign.SelectableFactions`（`SHE` / `Tenki` / `Sugar` / `Tuners`、`Neutral` は除外）に含まれない → 400 (`ErrInvalidFaction`)

処理（単一トランザクション）:

1. `player_factions (player_id, faction, 'initial_selection')` を `INSERT ... ON CONFLICT DO NOTHING`
2. INSERT が新規なら `players.selected_faction` を UPDATE
3. INSERT が既存なら `ErrFactionAlreadySelected` → 409

**冪等性の扱い**: 409 はエラーではなく「既に選択済み」のシグナル。クライアント（ゲートウェイ / UI）は 409 を成功と同等に扱う契約。

**Neutral を選べない理由**: Neutral はチュートリアル中のサブファクションで、プレイヤーが能動的に選ぶカテゴリではない。`gamedesign.SelectableFactions` が Neutral を含まないことで弾く。

### 5.2 ファクション付与 (`POST /internal/v1/players/:playerId/factions`)

運用・バックオフィス用途の直接付与。`{faction, source}` を受け取り `AddPlayerFaction` を呼ぶ。通常運用では使わず、`faction-selected` subscriber が主経路。

### 5.3 アクティブ選択の切り替え (`PUT /internal/v1/players/:playerId/faction`)

プレイヤーが所持している複数ファクションから「現在アクティブ」を切り替える。account 側では `players.selected_faction` の UPDATE のみ（所持判定は呼び出し元の責務）。

---

## 6. 経験値・レベル

### 6.1 `POST /internal/v1/players/award-game-exp`

battle サービスが試合終了時に呼ぶ唯一の経験値付与エンドポイント。

**入力**: `{player1_id, player2_id, winner_num, reason, match_type}`
**成功**: `204 No Content`

ルール:

| 条件 | player1 に付与 | player2 に付与 |
|---|---|---|
| `reason == "draw"` or `winner_num == 0` | `exp_draw` | `exp_draw`（`match_type=="npc"` ならスキップ） |
| `winner_num == 1` | `exp_win` | `exp_loss`（`match_type=="npc"` ならスキップ） |
| `winner_num == 2` | `exp_loss` | `exp_win`（`match_type=="npc"` ならスキップ） |

`match_type == "npc"` のとき player2 側は NPC として扱い exp を付与しない。

経験値量 (`exp_win` / `exp_loss` / `exp_draw`) と係数 (`exp_formula_coefficient`) は Firestore `game_config`。未設定・0 以下は 500。

### 6.2 `POST /internal/v1/players/:playerId/exp`

個別プレイヤーへの経験値加算。`AwardGameExp` の内部実装で呼ばれる共通処理と同じ `AddExp` を経由する。`exp_gain <= 0` なら no-op。

### 6.3 レベル計算の契約

- `ComputeLevel(newExp, currentLevel, coeff)` は **現在レベル以上にしか進まない**
- `nextLevelExp = coeff * (level+1)^2` のループで、`newExp >= nextLevelExp` の間レベルを +1
- 係数を厳しくしても既存レベルが下がらないことを保証する

---

## 7. プレミアムステータス

プレミアム状態の authoritative な SSoT は shop のサブスクリプション契約。account は射影を持つ。

### 7.1 書き込み経路

1. `premium-updated` Pub/Sub イベント（shop が publish）→ `players.is_premium` / `players.premium_expires_at` を UPDATE
2. `PUT /internal/v1/players/:playerId/premium` REST（内部呼び出し用の直接更新。運用・テスト用途）

本番運用では 1 のみが使われる想定。2 は back-door として残しているが、プレミアム状態の変更を account 直接更新で行ってはいけない（shop との不整合が起きる）。

### 7.2 `premium-updated` subscriber の冪等性

[ARCHITECTURE.md §5.3](ARCHITECTURE.md) の `processed_events` 契約に従う。同一 `event_id` は重複適用しない。

---

## 8. ユーザー設定

### 8.1 `GET /internal/v1/players/:playerId/settings`

`user_settings` を返す。行が存在しない場合は **アプリ層デフォルト値を返す** (404 にしない)。Register 時に INSERT するため通常は必ず存在するが、ガードとしてデフォルトを返す。

### 8.2 `PUT /internal/v1/players/:playerId/settings`

`{language, bgm_volume, se_volume, push_enabled}` で UPSERT。`language` が空なら 400。

音量範囲（0-100）のバリデーションは現在 DB CHECK / アプリ層で明示的に行っていない（BIGINT として受理）。将来の要件に応じて追加する。

---

## 9. Pub/Sub subscribe

publish 機能は持たない。subscribe するイベントは以下の 2 種。

### 9.1 `faction-selected`

subscription: `faction-selected-account-sub`

| publisher | source | account の副作用 |
|---|---|---|
| scenario | `scenario_initial` | `player_factions` INSERT + `players.selected_faction` UPDATE |
| shop | `shop_purchase` | `player_factions` INSERT のみ |

詳細契約: [ARCHITECTURE.md §5.1](ARCHITECTURE.md)

### 9.2 `premium-updated`

subscription: `premium-updated-account-sub`

- publisher: shop のみ
- 副作用: `players.is_premium` / `players.premium_expires_at` を UPDATE
- **shop が cancel 時に publish しない契約** のため、account 側で「解約即剥奪」を観測することはない

詳細契約: [ARCHITECTURE.md §5.2](ARCHITECTURE.md)

### 9.3 冪等性

両 subscriber とも `processed_events (event_id, event_type)` による ON CONFLICT DO NOTHING ガードで at-least-once を吸収する ([ARCHITECTURE.md §5.3](ARCHITECTURE.md))。

---

## 10. エラーセマンティクス

### 10.1 センチネルと HTTP ステータス

handler 層が `errors.Is` でセンチネルを分類して HTTP に変換する ([internal/handler/rest/errors.go](../internal/handler/rest/errors.go))。

| センチネル | HTTP | 契約 |
|---|---|---|
| `port.ErrNotFound` / `service.ErrPlayerNotFound` | 404 | リソース不在 |
| `service.ErrPlayerAlreadyRegistered` | 409 | Register は非冪等。gateway が Login にフォールバック |
| `service.ErrFactionAlreadySelected` | 409 | 冪等な成功扱い。クライアントはエラー表示しない |
| `service.ErrInvalidFaction` | 400 | 許可集合 (`gamedesign.SelectableFactions`) 外 |
| JSON bind 失敗 / 必須フィールド欠落 | 400 | ハンドラ手前で弾く |
| その他 wrap されたエラー | 500 | DB 接続断・Firestore 読み取り失敗等 |

### 10.2 握りつぶし禁止

DB エラー・Pub/Sub デシリアライズ失敗・Firestore 読み取り失敗をログだけで握りつぶさない（CLAUDE.md 設計思想）。subscriber の「未知 event_type を ACK」は握りつぶしではなく「責務外として意図的に skip」で、ログは残す。

### 10.3 フォールバック禁止

Firestore の `game_config` キー未設定（値 0）はデフォルト値で継続せず、当該リクエストをエラーにする。運用者が値を入れるまでサービスが動かない方が、意図しない経験値付与より安全、という選択。

---

## 11. イベント契約

**account はイベントを publish しない**。状態遷移は REST レスポンスで通知するか、他サービス（shop / scenario）が publish するイベントを subscribe するのみ。

この非対称性は ADR-011 / ADR-014 の schema ownership 契約から来ている。account の下流サービスは存在しない（account の DB 行を「見たい」サービスは REST 経由で都度取得するか、shop のように自前で `player_owned_factions` のような read model を持つ）。
