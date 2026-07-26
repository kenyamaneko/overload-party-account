# Account 機能仕様書

このドキュメントは account サービスがビジネス要件として **何を保証するか** を記述する。実装方法ではなく振る舞いの契約を定義するため、テストは本書の観点で書く。

関連ドキュメント:
- 内部動作・配線・運用設定: [ARCHITECTURE.md](ARCHITECTURE.md)
- HTTP エンドポイント契約 (SSoT): [../data/openapi.yaml](../data/openapi.yaml)
- DB スキーマ: [DATA_DESIGN.md](DATA_DESIGN.md)

---

## サービス責務

account は以下の機能ドメインを所有する。

| 機能 | 主要な責務 |
|---|---|
| プレイヤー登録・ログイン | Firebase UID と player_id の紐付け。初期行の作成（players / player_progression / player_settings）。表示名はオンボーディング完了時に別経路で確定するため Register 時には受け取らない |
| プレイヤー情報の参照・更新 | プレイヤー名・選択ファクション・装備アイコンの参照/更新。レベル進捗の算出 |
| デイリーバトル制限 | JST 05:00 境界の制限回数管理。increment は非冪等で、二重呼び出し防止は battle 側の責務 |
| ファクション所有 | オンボード内 faction 選択時の初期 faction 登録（`onboarding-faction-set` イベント）と shop 購入時の追加（`faction-acquired` イベント）を `player_factions` に射影 |
| 経験値・レベル | `AwardGameExp` による両プレイヤー同時付与。係数変更時もレベルは下がらない |
| プレミアムステータス | `premium-updated` イベントから `is_premium` を射影保持 |
| 表示名の検証・反映 | `PUT /api/v1/account/me/name` で表示名を受け、業務ルール (空・空白のみ・制御文字・`MaxNameRunes` 超) を [domain.ValidateName](../internal/domain/name.go) で検証して `players.name` を UPDATE |
| ユーザー設定 | 言語・音量・通知フラグの参照/更新 |

account は **account スキーマの DB 行と Pub/Sub から取り込んだ状態を唯一の真実とし**、他サービスを直接呼び出さず、自らイベントを publish もしない。

---

## Register / Login

### `POST /internal/v1/auth/register`

**入力**: `{firebase_uid}`（gateway が ID Token 検証済み前提）
**成功**: `201 Created` + `Player` レスポンス（`name` は `null`）
**処理**:

1. `firebase_uid` で既存プレイヤーを pre-check。存在すれば 409 (`ErrPlayerAlreadyRegistered`)
2. 新 UUID を採番し、単一トランザクションで:
   - `players` INSERT (`name` NULL, is_premium=false)
   - `player_progression` INSERT (level=1, exp=0)
   - `player_settings` INSERT (アプリ層デフォルト値)

`player_daily_battle` は Register では作らない。1 行/プレイヤー/ゲーム日の履歴台帳で、初回バトルの `IncrementDailyBattleCount` UPSERT で発生する (詳細は「デイリーバトル制限」)。

`name` は本エンドポイントでは受け取らない。表示名はオンボーディングシナリオの中で
ユーザーが入力した時点で `POST /api/v1/account/me/onboarding/name/validate` で検証され、
scenario が publish する `onboarding-name-set` イベント (「`onboarding-name-set`」) で
`players.name` に確定する (`MaxNameRunes` などの業務バリデーション SSoT は account)。
これによりオンボーディング途中で離脱しても再 Register を強要せず、シナリオ再開時は
`players.name` の有無を見て入力ステップをスキップ判定できる。

### Register の冪等性契約

- **冪等ではない**。同一 `firebase_uid` の二重 Register は 409 を返す
- gateway 側で「Register を試して 409 なら Login にフォールバック」する契約を採用している。account 単体では吸収しない
- スターターカード配布・初期ファクション選択は Register に含まれない（[ARCHITECTURE.md](ARCHITECTURE.md) の「なぜスターターカードや初期ファクションを含めないか」）

### `POST /internal/v1/auth/login`

**入力**: `{firebase_uid}`
**成功**: `200 OK` + `Player` レスポンス
**処理**: `players` を `firebase_uid` で検索。不在なら 404 (`ErrPlayerNotFound`)

副作用なし。

### `GET /internal/v1/auth/by-firebase-uid/:firebaseUID`

**入力**: パスパラメータ `firebaseUID`
**成功**: `200 OK` + `Player` レスポンス
**処理**: `players` を `firebase_uid` で検索。不在なら 404

Login と同じルックアップだが、ログインという業務イベントを伴わない参照系（gateway などサービス間の UID→Player 解決用）。

---

## プレイヤー情報の参照・更新

| メソッド | パス | 概要 |
|---|---|---|
| GET | `/api/v1/account/me` | プレイヤー情報 + レベル進捗を返す |
| PUT | `/api/v1/account/me/name` | name 更新 |
| POST | `/api/v1/account/me/onboarding/name/validate` | オンボード内 name 入力ステップでの表示名バリデーション（書き込みなし） |
| PUT | `/api/v1/account/me/premium` | プレミアムステータス更新（battle 等の内部呼び出し用） |
| POST | `/api/v1/account/me/factions` | ファクションの明示的付与（運用用） |
| GET | `/api/v1/account/me/factions` | 所持ファクション一覧 |
| POST | `/api/v1/account/me/factions/select` | initial faction 選択 (オンボーディング経路。詳細は「初期ファクション選択」) |

### `GetPlayerResponse` のレベル進捗

GetPlayer レスポンスには現在レベル内の `level_exp_current` と `level_exp_required` を含める。

- `level_exp_current = max(0, exp - coeff * level * level)`
- `level_exp_required = coeff * (level+1) * (level+1) - coeff * level * level`
- `coeff` は Firestore `game_config.exp_formula_coefficient`。未設定（0 以下）は 500 エラー

---

## デイリーバトル制限

### ゲーム日の境界

ゲーム日は **JST 05:00 = UTC 20:00** にリセットする。実装は `time.Now().UTC().Add(4h)` の日付部分を取る（[ARCHITECTURE.md](ARCHITECTURE.md) の「ゲーム日の境界 (JST 05:00)」）。

### `GET /api/v1/account/me/battle-limit`

`BattleLimitResponse` を返す:

| フィールド | 意味 |
|---|---|
| `daily_battle_count` | 当日のゲーム日に対応する `player_daily_battle` 行のカウント。行が無ければ 0 |
| `daily_battle_limit` | free の上限。プレミアム時は `-1`（無制限シグナル） |
| `can_battle` | `daily_battle_count < daily_battle_limit`。プレミアム時は常に true |

上限値は Firestore `game_config.free_daily_battle_limit`。値 0（未設定）は 500 エラー。

### `POST /api/v1/account/me/battle-limit/increment`

battle サービスが試合開始時に呼ぶ。

- **プレミアム会員でもカウントを記録する**（上限判定は行わない）
- 当日のゲーム日 `(player_id, game_date)` を 1 SQL で UPSERT。行が無ければ count=1 で発生し、あれば +1 加算
- レスポンスは 204 No Content

冪等ではない（呼ぶたびにインクリメント）。同一試合に対する二重呼び出しを避けるのは battle 側の責務。

### `POST /internal/v1/players/revert-battle-count`

gateway の停止時処理が対戦単位で呼ぶ。停止によって無効になった対戦のために消費したバトル回数を両プレイヤーに戻す。

**入力**: `{game_id, player1_id, player2_id, consumed_at_millis}`
**成功**: `204 No Content`

- 両プレイヤーの消費バトル回数を戻す
- 対象のゲーム日は `consumed_at_millis`（バトル回数を消費した時刻）から算出する。停止が発生した時刻ではなく、消費した時点の時刻を渡す契約
- 同一 `game_id` の呼び出しは 1 回のみ適用される。2 回目以降は何もせず成功を返す
- カウントの下限は 0。対象のゲーム日に記録が無い、または既に 0 であっても負の値にはならず成功を返す

---

## ファクション所有

### 初期ファクション選択 (`POST /api/v1/account/me/factions/select`)

オンボーディングで最初に選択した faction を保持する。

**入力**: `{faction_id}`
**成功**: `200 OK`

バリデーション:

1. `playerId` が空 → 400
2. `faction_id` が空 → 400
3. `faction_id` が `gamedesign.SelectableFactions`（`SHE` / `Tenki` / `Sugar` / `Tuners`、`Neutral` は除外）に含まれない → 400 (`ErrInvalidFaction`)
4. プレイヤー不在 → 404
5. 既に選択済み → 409 (`ErrFactionAlreadySelected`、クライアントは 409 を成功と同等に扱う契約)

### ファクション付与 (`POST /api/v1/account/me/factions`)

運用・バックオフィス用途の直接付与。`{faction}` を受け取り `AddPlayerFaction` を呼ぶ (`is_initial=FALSE` 固定)。

---

## 経験値・レベル

### `POST /internal/v1/players/award-game-exp`

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

### `POST /api/v1/account/me/exp`

個別プレイヤーへの経験値加算。`AwardGameExp` の内部実装で呼ばれる共通処理と同じ `AddExp` を経由する。`exp_gain <= 0` なら no-op。

### レベル計算の契約

- `ComputeLevel(newExp, currentLevel, coeff)` は **現在レベル以上にしか進まない**
- `nextLevelExp = coeff * (level+1)^2` のループで、`newExp >= nextLevelExp` の間レベルを +1
- 係数を厳しくしても既存レベルが下がらないことを保証する

---

## プレミアムステータス

プレミアム状態の authoritative な SSoT は shop のサブスクリプション契約。account は射影を持つ。

### 書き込み経路

1. `premium-updated` Pub/Sub イベント（shop が publish）→ `players.is_premium` / `players.premium_expires_at` を UPDATE
2. `PUT /api/v1/account/me/premium` REST（内部呼び出し用の直接更新。運用・テスト用途）

本番運用では 1 のみが使われる想定。2 は back-door として残しているが、プレミアム状態の変更を account 直接更新で行ってはいけない（shop との不整合が起きる）。

### `premium-updated` subscriber の冪等性

[ARCHITECTURE.md](ARCHITECTURE.md) の「`processed_events` による冪等性契約」に従う。同一 `event_id` は重複適用しない。

---

## ユーザー設定

### `GET /api/v1/account/me/settings`

`player_settings` を返す。Register と同一トランザクションで必ず INSERT される契約のため、行が存在しないのは Register 未実施または不整合の症状。デフォルト値で隠さず **404 (`port.ErrNotFound`)** を返す。

### `PUT /api/v1/account/me/settings`

部分更新。body の各フィールドは **ポインタ型** で、省略（nil / JSON で存在しないキー）は「変更なし」を意味する:

```json
{ "language": "en" }                       // language だけ変更、他は現状維持
{ "bgm_volume": 0, "push_enabled": false } // 音量ミュート + 通知オフ
```

- 1 つもフィールドを指定していない空 body は 400
- `player_settings` 行が存在しないプレイヤーは 404（通常 Register で INSERT 済み）
- SQL は `COALESCE` ベース。詳細契約は [ARCHITECTURE.md](ARCHITECTURE.md) の「プレイヤー設定の部分更新契約」

音量範囲（0-100）のバリデーションは現在 DB CHECK / アプリ層で明示的に行っていない（BIGINT として受理）。将来の要件に応じて追加する。

---

## Pub/Sub subscribe

publish 機能は持たない。subscribe するイベントは以下の 5 種。

### `faction-acquired`

subscription: `faction-acquired-account-sub`

| publisher | account の副作用 |
|---|---|
| shop | `player_factions` INSERT のみ (`is_initial=FALSE` 固定) |

ADR-022 により、かつて `faction-selected` topic が担っていた 2 業務事実（scenario 初期選択 / shop 購入）は業務事実単位で分解された。scenario 初期選択は「`onboarding-faction-set`」に統合され、本 topic は shop 購入のみを扱う。さらに ADR-031 で shop 側の `faction-purchased` を `card-pack-purchased` (card 向け) と `faction-acquired` (本 topic) に分割。

詳細契約: [ARCHITECTURE.md](ARCHITECTURE.md) の「faction-acquired subscriber」

### `premium-updated`

subscription: `premium-updated-account-sub`

- publisher: shop のみ
- 副作用: `players.is_premium` / `players.premium_expires_at` を UPDATE
- **shop が cancel 時に publish しない契約** のため、account 側で「解約即剥奪」を観測することはない

詳細契約: [ARCHITECTURE.md](ARCHITECTURE.md) の「premium-updated subscriber」

### `onboarding-name-set`

subscription: `onboarding-name-set-account-sub`

- publisher: scenario のみ（オンボード内 name 入力ステップで、account の validate REST 成功後に publish）
- 副作用: `players.name` UPDATE と `onboarding_status='name_set'` への前進を 1 tx で実行

詳細契約: [ARCHITECTURE.md](ARCHITECTURE.md) の「onboarding-name-set subscriber」

### `onboarding-faction-set`

subscription: `onboarding-faction-set-account-sub`

- publisher: scenario のみ（オンボード内 faction 選択ステップの検証成功後に publish）
- 副作用: `player_factions` への initial faction 反映 (`is_initial=TRUE`) と `onboarding_status='faction_set'` への前進を 1 tx で実行

詳細契約: [ARCHITECTURE.md](ARCHITECTURE.md) の「onboarding-faction-set subscriber」

### `player-onboarded`

subscription: `player-onboarded-account-sub`

- publisher: scenario のみ（オンボーディングシナリオ読了時に transactional outbox 経由で publish、[ADR-021](../../overload-party-common/docs/adr/021-onboarding-scenario.md)、ADR-022 で 1 イベント設計に縮退）
- 副作用: `players.onboarding_status='completed'` への遷移のみ
- 表示名 (`players.name`) は本経路では扱わない (入力時点で先行する `onboarding-name-set` subscriber が確定する設計)
- initial faction の永続化は先行する `onboarding-faction-set` subscriber (「`onboarding-faction-set`」、[ARCHITECTURE.md](ARCHITECTURE.md) の「onboarding-faction-set subscriber」) が完了している前提

詳細契約: [ARCHITECTURE.md](ARCHITECTURE.md) の「player-onboarded subscriber」

### 冪等性

全 subscriber とも `processed_events (event_id, event_type)` による ON CONFLICT DO NOTHING ガードで at-least-once を吸収する ([ARCHITECTURE.md](ARCHITECTURE.md) の「`processed_events` による冪等性契約」)。

---

## エラーセマンティクス

### センチネルと HTTP ステータス

handler 層が `errors.Is` でセンチネルを分類して HTTP に変換する ([internal/handler/rest/errors.go](../internal/handler/rest/errors.go))。

| センチネル | HTTP | 契約 |
|---|---|---|
| `port.ErrNotFound` / `usecase.ErrPlayerNotFound` | 404 | リソース不在 |
| `usecase.ErrPlayerAlreadyRegistered` | 409 | Register は非冪等。gateway が Login にフォールバック |
| `usecase.ErrFactionAlreadySelected` | 409 | 冪等な成功扱い。クライアントはエラー表示しない |
| `usecase.ErrInvalidFaction` | 400 | 許可集合 (`gamedesign.SelectableFactions`) 外 |
| `domain.ErrInvalidName` | 400 | 表示名の業務ルール (空・空白のみ・制御文字・`MaxNameRunes` 超) 違反 |
| JSON bind 失敗 / 必須フィールド欠落 | 400 | ハンドラ手前で弾く |
| その他 wrap されたエラー | 500 | DB 接続断・Firestore 読み取り失敗等 |

### 握りつぶし禁止

DB エラー・Pub/Sub デシリアライズ失敗・Firestore 読み取り失敗をログだけで握りつぶさない（CLAUDE.md 設計思想）。subscriber の「未知 event_type を ACK」は握りつぶしではなく「責務外として意図的に skip」で、ログは残す。

### フォールバック禁止

Firestore の `game_config` キー未設定（値 0）はデフォルト値で継続せず、当該リクエストをエラーにする。運用者が値を入れるまでサービスが動かない方が、意図しない経験値付与より安全、という選択。

---

## イベント契約

**account はイベントを publish しない**。状態遷移は REST レスポンスで通知するか、他サービス（shop / scenario）が publish するイベントを subscribe するのみ。

この非対称性は ADR-011 / ADR-014 の schema ownership 契約から来ている。account の下流サービスは存在しない（account の DB 行を「見たい」サービスは REST 経由で都度取得するか、shop のように自前で `player_owned_factions` のような read model を持つ）。
