# account スキーマ - データ設計

> **DDL の SSoT:** `db/schema.sql`
> **テーブル内カラム表の再生成:** `python3 scripts/generate_schema_doc.py` が `<!-- BEGIN/END GENERATED -->` マーカー内を上書きする。マーカー外の設計判断・リレーション図は手動で保守する。

## 設計概要

account スキーマはプレイヤーの基本情報・デイリーバトル回数・ファクション所有・ユーザー設定・冪等性レコード (Pub/Sub subscriber 用と REST 呼び出し用) を所有する。全サービスの中で最も多くの外部参照を受けるスキーマだが、他サービスからの直接 SELECT は許容せず account の内部 REST API 経由でのみ公開する（ADR-011 / ADR-014）。

`is_premium` / `premium_expires_at` は shop が authoritative な状態を射影しているだけで、account 内では read-only 扱い（書き込みは `premium-updated` subscriber と、運用・テスト用の `PUT /api/v1/account/me/premium` のみ）。

ゲームバランス調整値 (デイリーバトル無料上限・経験値係数など) は account スキーマに持たず、Cloud Firestore `game_config` を読み取り専用で参照する（[ADR-017](https://github.com/kenyamaneko/overload-party-common/blob/main/docs/adr/017-game-config-firestore.md)）。ファクションマスターも DB に持たず、`common/data/factions.yaml` から code-generate された定数を参照する。

---

## テーブル構成

### players

プレイヤーマスター。Firebase Auth UID と 1:1 で対応する。

- **PK:** `player_id` (UUID)
- **UNIQUE INDEX:** `idx_players_firebase_uid` ON `firebase_uid`
- **CHECK:** `onboarding_status IN ('not_started', 'name_set', 'faction_set', 'completed')`
- **TRIGGER:** `trg_players_updated_at`：UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: players -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | UUID |
| `firebase_uid` | VARCHAR(128) | No | Firebase Auth UID (Unique) |
| `name` | VARCHAR(50) | Yes | 表示名 (NULL: 未設定) |
| `is_premium` | BOOLEAN | No | 課金ステータス |
| `equipped_icon_no` | BIGINT | Yes | 装備中アイコン番号（NULL: デフォルト） |
| `onboarding_status` | VARCHAR(20) | No | オンボード進行状態 |
| `premium_expires_at` | TIMESTAMPTZ | Yes | サブスク有効期限 |
| `created_at` | TIMESTAMPTZ | No | 作成日時 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: players -->

**設計判断:**
- `is_premium` / `premium_expires_at` を players に射影で持つ理由は、ほぼ全ての REST レスポンスで課金ステータスを返す必要があり、毎回 shop を呼ぶと結合が強くなりすぎるため。shop が authoritative で、`premium-updated` Pub/Sub で最終的整合させる
- 「オンボーディングで選択した faction」は `players` 側に列を置かず、`player_factions.is_initial=TRUE` の行が SSoT (「player_factions」参照)
- `name` は「表示名」。Register 時には NULL で挿入し、オンボーディング中の `onboarding-name-set` イベント (subscriber) で確定する。account 側で `display_name` 列を別途設けない（同一セマンティクスの列を複数持つと SSoT が分散するため）
- `equipped_icon_no` は shop 側の `cosmetic_items(item_type='icon', item_no=N)` を参照するが、cross-schema FK は張らない（アプリ層整合性）

### player_daily_battle

デイリーバトル回数の履歴台帳。1 行 = 1 プレイヤー × 1 ゲーム日。

- **PK:** `(player_id, game_date)` (`player_id` → `players.player_id`, ON DELETE CASCADE)

<!-- BEGIN GENERATED: player_daily_battle -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `game_date` | DATE | No | ゲーム日 (JST 05:00 リセット) |
| `daily_battle_count` | BIGINT | No | そのゲーム日のバトル回数 |
<!-- END GENERATED: player_daily_battle -->

**設計判断:**
- players に埋め込まず別テーブルにしているのは、バトル回数チェック / increment が高頻度で走るのに対し、players 本体の更新 (name / is_premium 等) とは独立しているため。更新競合を分離する目的
- リセット境界は JST 05:00。`game_date` は usecase 層の `gameDayFor(t)` が算出する civil.Date
- 1 行/日で履歴を残すのは、将来 BigQuery エクスポートでプレイヤーごとの日次バトル回数を分析できるようにするため。これがアプリ内で履歴が残る唯一の場所
- 当日の行が無ければカウント 0 とみなす (新ゲーム日でまだバトルしていない状態)。Register 時には INSERT せず、初回バトルの UPSERT で行が発生する

### battle_count_reversals

停止で無効になった対戦の消費バトル回数返却が、対戦単位で一度しか適用されないことを保証するアプリ層ガードテーブル。

- **PK:** `game_id` (TEXT)
- **FK なし**（`game_id` は battle が発行する対戦識別子で、account の管理下にない）

<!-- BEGIN GENERATED: battle_count_reversals -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `game_id` | TEXT | No | 対戦識別子 (battle が発行する game_id) |
| `reverted_at` | TIMESTAMPTZ | No | 返却処理日時 |
<!-- END GENERATED: battle_count_reversals -->

**設計判断:**
- `processed_events` は Pub/Sub subscriber の冪等性専用ガードのため流用せず、REST 呼び出しの冪等性は別テーブルに持つ
- `INSERT ... ON CONFLICT DO NOTHING RETURNING game_id` で重複呼び出しを検知する。`processed_events` と同じパターンだが対象イベントの性質が異なるためテーブルを分ける
- 返却対象の `game_date` は保持しない。呼び出し側が渡す消費時刻から都度算出するため、本テーブルは「同じ対戦を二度処理しない」ことだけを保証する
- `game_id` は UUID 型にせず TEXT にしている。account は battle が発行する識別子の具体的なフォーマットを持たないため、フォーマットの前提を account 側の DDL に持ち込まない

### player_factions

プレイヤーが所持しているファクション (カードセット) と「オンボーディングで選択した faction」を兼ねるテーブル。

- **PK:** `(player_id, faction)`
- **FK:** `player_id` → `players` (ON DELETE CASCADE)
- **CHECK:** `faction IN ('SHE', 'Tenki', 'Sugar', 'Tuners', 'Neutral')`
- **PARTIAL UNIQUE INDEX:** `idx_player_factions_initial` ON `(player_id) WHERE is_initial = TRUE` (1 プレイヤーに initial faction は最大 1 つ)

<!-- BEGIN GENERATED: player_factions -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `faction` | VARCHAR(20) | No | 陣営名 (SHE / Tenki / Sugar / Tuners / Neutral) |
| `is_initial` | BOOLEAN | No | オンボーディングで選択した faction か (1 プレイヤーにつき最大 1 行 TRUE) |
| `acquired_at` | TIMESTAMPTZ | No | 取得日時 |
<!-- END GENERATED: player_factions -->

**設計判断:**
- 所持リストと「オンボーディングで選択した faction」を 1 テーブルに集約。`is_initial=TRUE` の行が後者で、partial unique index で「1 プレイヤーに最大 1 つ」を DB レベルで保証する
- ショップ先行で買った行をオンボーディングで initial 確定するときは、同じ行を `is_initial=TRUE` に昇格させる
- 複合 PK `(player_id, faction)` が冪等性のキー
- `factions` リファレンステーブルは存在しない。ファクションマスターの SSoT は `common/data/factions.yaml` から code-generate された定数で、DB 側では CHECK 制約で enum を表現する

### player_settings

プレイヤー設定。players と 1:1。

- **PK:** `player_id` (→ `players.player_id`, ON DELETE CASCADE)
- **TRIGGER:** `trg_player_settings_updated_at`：UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: player_settings -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | プレイヤーID |
| `"language"` | VARCHAR(10) | No | 言語設定 |
| `bgm_volume` | BIGINT | No | BGM音量 (0-100) |
| `se_volume` | BIGINT | No | SE音量 (0-100) |
| `push_enabled` | BOOLEAN | No | 通知許可 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: player_settings -->

**設計判断:**
- デフォルト値は DB 側の DEFAULT ではなくアプリ層 (`internal/domain/defaults.go`) で制御する。理由は言語判定をクライアントの Accept-Language 等と揃える余地を残すため
- 登録時（Register）に `player_settings` 行をアプリ層デフォルトで INSERT する

### player_progression

レベルと経験値。players と 1:1 の子テーブル。

- **PK:** `player_id` (→ `players.player_id`, ON DELETE CASCADE)
- **TRIGGER:** `trg_player_progression_updated_at`：UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: player_progression -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `level` | BIGINT | No | レベル (Default: 1) |
| `exp` | BIGINT | No | 経験値 (Default: 0) |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: player_progression -->

**設計判断:**
- かつては `players.level` / `players.exp` として同居していたが、戦闘ごとの高頻度 UPDATE が `players.updated_at` を押し上げプロフィール変更の検知を汚染する・`players` 全体の SELECT FOR UPDATE で他 UPDATE と競合する・MVCC dead tuple が `players` に集中する、などの理由で分離した（[ADR-068](https://github.com/kenyamaneko/overload-party-common/blob/main/docs/adr/068-account-player-progression-separate-table.md)）
- API レスポンスの `Player` には引き続き `level` / `exp` を含めるため、repository 層で JOIN して Player アグリゲートに詰めて返す
- 書き込みホットパス（`AddExp`）は `player_progression` のみを触る。`players` は静かなまま

### processed_events

Pub/Sub subscriber の冪等性を保証するアプリ層ガードテーブル。

- **PK:** `event_id` (UUID)
- **FK なし**（players とはライフサイクルが独立）

<!-- BEGIN GENERATED: processed_events -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `event_id` | UUID | No | Pub/Sub EventID (publisher 生成の UUIDv4) |
| `event_type` | TEXT | No | イベント種別 (faction_acquired / premium_updated / player_onboarded) |
| `processed_at` | TIMESTAMPTZ | No | 処理日時 |
<!-- END GENERATED: processed_events -->

**設計判断:**
- account は Pub/Sub の push subscription で受信する。push は at-least-once 配信のみで exactly-once をサポートしないため、本テーブルが冪等性を保証する唯一の防御になる
- subscriber は `INSERT ... ON CONFLICT DO NOTHING RETURNING event_id` で重複を検知。`RETURNING` が空なら処理本体をスキップして成功として応答する
- 処理本体と INSERT は同一トランザクション内で完結するため、「適用されたのに event_id が記録されない」状態は構造的に発生しない

---

## リレーション

```
players (PK: player_id)
  │
  ├── 1:1 ── player_progression  (FK: player_id, CASCADE)
  ├── 1:N ── player_daily_battle (FK: player_id, CASCADE)  -- 1 行/ゲーム日
  ├── 1:N ── player_factions     (FK: player_id, CASCADE)
  └── 1:1 ── player_settings     (FK: player_id, CASCADE)

processed_events (独立、FK なし)
battle_count_reversals (独立、FK なし)

[shop.subscriptions] ─ ─ ─ (cross-schema, app-level via premium-updated)
        └─→ players.is_premium / premium_expires_at  (射影)

[shop.player_owned_factions] ─ ─ ─ (cross-schema read model)
        ←─  player_factions  (authoritative)
```

点線は cross-schema / cross-service の app-level 整合。DB の外部キーは張らず、Pub/Sub イベントと subscriber の冪等性で最終的整合させる。

---

## インデックス戦略

| インデックス | 対象 | 用途 |
|---|---|---|
| `idx_players_firebase_uid` (UNIQUE) | `players(firebase_uid)` | ログイン時 Firebase UID → player_id lookup。認証フローの起点 |

他のセカンダリインデックスは現状定義していない。PK（`player_id` / 複合 PK）が主要クエリパスをカバーする。将来 `player_factions` からの「ファクション別プレイヤー数」集計などが必要になったら `player_factions(faction)` を検討する。
