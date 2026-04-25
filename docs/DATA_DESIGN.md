# account スキーマ - データ設計

> **DDL の SSoT:** `db/schema.sql`
> **テーブル内カラム表の再生成:** `python3 scripts/generate_schema_doc.py` が `<!-- BEGIN/END GENERATED -->` マーカー内を上書きする。マーカー外の設計判断・リレーション図は手動で保守する。

## 設計概要

account スキーマはプレイヤーの基本情報・デイリーバトル回数・ファクション所有・ユーザー設定・Pub/Sub 冪等性レコードを所有する。全サービスの中で最も多くの外部参照を受けるスキーマだが、他サービスからの直接 SELECT は許容せず account の内部 REST API 経由でのみ公開する（ADR-011 / ADR-014）。

`is_premium` / `premium_expires_at` は shop が authoritative な状態を射影しているだけで、account 内では read-only 扱い（書き込みは `premium-updated` subscriber からのみ）。

---

## テーブル構成

### 1. players

プレイヤーマスター。Firebase Auth UID と 1:1 で対応する。

- **PK:** `player_id` (UUID)
- **UNIQUE INDEX:** `idx_players_firebase_uid` ON `firebase_uid`
- **CHECK:** `selected_faction IS NULL OR selected_faction IN ('SHE', 'Tenki', 'Sugar', 'Tuners', 'Neutral')`
- **TRIGGER:** `trg_players_updated_at` — UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: players -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | UUID |
| `firebase_uid` | VARCHAR(128) | No | Firebase Auth UID (Unique) |
| `name` | VARCHAR(50) | No | 表示名 |
| `is_premium` | BOOLEAN | No | 課金ステータス |
| `equipped_icon_no` | BIGINT | Yes | 装備中アイコン番号（NULL: デフォルト） |
| `selected_faction` | VARCHAR(20) | Yes | 選択済みファクション |
| `premium_expires_at` | TIMESTAMPTZ | Yes | サブスク有効期限 |
| `created_at` | TIMESTAMPTZ | No | 作成日時 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: players -->

**設計判断:**
- `is_premium` / `premium_expires_at` を players に射影で持つ理由は、ほぼ全ての REST レスポンスで課金ステータスを返す必要があり、毎回 shop を呼ぶと結合が強くなりすぎるため。shop が authoritative で、`premium-updated` Pub/Sub で最終的整合させる
- `selected_faction` は「現在アクティブなファクション」。オンボーディング完了時に `player-onboarded` イベント受信で初期値が入り（ADR-022 により `faction-selected` 経由ではなく `player-onboarded` に統合済み）、以降は `PUT /players/:id/faction` でプレイヤーが切り替える
- `name` は「表示名」。Register 時の値を、オンボーディング完了イベント (`player-onboarded`、scenario が publish) で上書きする運用。account 側で `display_name` 列を別途設けない（同一セマンティクスの列を複数持つと SSoT が分散するため）
- `equipped_icon_no` は shop 側の `cosmetic_items(item_type='icon', item_no=N)` を参照するが、cross-schema FK は張らない（アプリ層整合性）

### 2. player_daily_battle

デイリーバトル回数の管理。players と 1:1。

- **PK:** `player_id` (→ `players.player_id`, ON DELETE CASCADE)

<!-- BEGIN GENERATED: player_daily_battle -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `daily_battle_count` | BIGINT | No | 本日のバトル回数 |
| `last_reset_date` | DATE | No | 最終リセット日 |
<!-- END GENERATED: player_daily_battle -->

**設計判断:**
- players に埋め込まず別テーブルにしているのは、バトル回数チェック / increment が高頻度で走るのに対し、players 本体の更新（name / is_premium 等）とは独立しているため。更新競合を分離する目的
- リセット日境界は JST 05:00。詳細は [ARCHITECTURE.md §4.1](ARCHITECTURE.md)
- `last_reset_date` は「最後にカウンタを 0 に戻した日」。Increment 時に当日のゲーム日と比較してリセット判定する

### 3. player_factions

プレイヤーが所持しているファクション（カードセット）の中間テーブル。

- **PK:** `(player_id, faction)`
- **FK:** `player_id` → `players` (ON DELETE CASCADE)
- **CHECK:** `faction IN ('SHE', 'Tenki', 'Sugar', 'Tuners', 'Neutral')`
- **CHECK:** `source IN ('initial_selection', 'shop_purchase')`

<!-- BEGIN GENERATED: player_factions -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `faction` | VARCHAR(20) | No | 陣営名 (SHE / Tenki / Sugar / Tuners / Neutral) |
| `source` | VARCHAR(20) | No | 取得経路 (initial_selection / shop_purchase) |
| `acquired_at` | TIMESTAMPTZ | No | 取得日時 |
<!-- END GENERATED: player_factions -->

**設計判断:**
- `players.selected_faction` は「今アクティブなファクション」、`player_factions` は「所持している全ファクション」。両者は責務が違うため分離している
- 書き込み経路は 2 つ: `player-onboarded` subscriber（scenario が publish、initial_selection）と `faction-purchased` subscriber（shop が publish、shop_purchase）。運用 REST (`POST /players/:id/factions`) は直接付与用のバックアップ経路
- 複合 PK `(player_id, faction)` が冪等性のキー。`INSERT ... ON CONFLICT DO NOTHING` で重複適用を排除する
- `factions` リファレンステーブルは存在しない。ファクションマスターの SSoT は `common/data/factions.yaml` から code-generate された定数で、DB 側では CHECK 制約で enum を表現する

### 4. player_settings

プレイヤー設定。players と 1:1。

- **PK:** `player_id` (→ `players.player_id`, ON DELETE CASCADE)
- **TRIGGER:** `trg_player_settings_updated_at` — UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: player_settings -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | プレイヤーID |
| `language` | VARCHAR(10) | No | 言語設定 |
| `bgm_volume` | BIGINT | No | BGM音量 (0-100) |
| `se_volume` | BIGINT | No | SE音量 (0-100) |
| `push_enabled` | BOOLEAN | No | 通知許可 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: player_settings -->

**設計判断:**
- デフォルト値は DB 側の DEFAULT ではなくアプリ層 (`internal/model/defaults.go`) で制御する。理由は言語判定をクライアントの Accept-Language 等と揃える余地を残すため
- 登録時（Register）に `player_settings` 行をアプリ層デフォルトで INSERT する

### 5. player_progression

レベルと経験値。players と 1:1 の子テーブル。

- **PK:** `player_id` (→ `players.player_id`, ON DELETE CASCADE)
- **TRIGGER:** `trg_player_progression_updated_at` — UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: player_progression -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `level` | BIGINT | No | レベル (Default: 1) |
| `exp` | BIGINT | No | 経験値 (Default: 0) |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: player_progression -->

**設計判断:**
- かつては `players.level` / `players.exp` として同居していたが、戦闘ごとの高頻度 UPDATE が `players.updated_at` を押し上げプロフィール変更の検知を汚染する・`players` 全体の SELECT FOR UPDATE で他 UPDATE と競合する・MVCC dead tuple が `players` に集中する、などの理由で分離した（[ARCHITECTURE.md §1.3](ARCHITECTURE.md)）
- API レスポンスの `Player` には引き続き `level` / `exp` を含めるため、repository 層で JOIN して Player アグリゲートに詰めて返す
- 書き込みホットパス（`AddExp`）は `player_progression` のみを触る。`players` は静かなまま

### 6. processed_events

Pub/Sub subscriber の冪等性を保証するアプリ層ガードテーブル。

- **PK:** `event_id` (UUID)
- **FK なし**（players とはライフサイクルが独立）

<!-- BEGIN GENERATED: processed_events -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `event_id` | UUID | No | Pub/Sub EventID (publisher 生成の UUIDv4) |
| `event_type` | TEXT | No | イベント種別 (faction_purchased / premium_updated / player_onboarded) |
| `processed_at` | TIMESTAMPTZ | No | 処理日時 |
<!-- END GENERATED: processed_events -->

**設計判断:**
- Pub/Sub の Exactly-Once Delivery に対するアプリ層の二重防御。Exactly-Once 契約が破れたとき（再配信・手動 replay）のセーフティネット
- subscriber は `INSERT ... ON CONFLICT DO NOTHING RETURNING event_id` で重複を検知。`RETURNING` が空なら処理本体をスキップして ACK する
- 処理本体と INSERT は同一トランザクション内で完結するため、「適用されたのに event_id が記録されない」状態は構造的に発生しない

---

## リレーション

```
players (PK: player_id)
  │
  ├── 1:1 ── player_progression  (FK: player_id, CASCADE)
  ├── 1:1 ── player_daily_battle (FK: player_id, CASCADE)
  ├── 1:N ── player_factions     (FK: player_id, CASCADE)
  └── 1:1 ── player_settings     (FK: player_id, CASCADE)

processed_events (独立、FK なし)

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
