# account スキーマ - データ設計

> **DDL の SSoT:** `db/schema.sql`

## 設計概要

account スキーマはプレイヤーの基本情報・デイリーバトル管理・陣営所持・設定を管理する。全サービスの中で最も多くの外部参照を受けるスキーマだが、他サービスからの直接 SELECT は許容せず account の REST API 経由でのみ提供する。

---

## テーブル構成

### players

プレイヤーマスター。Firebase Auth UID と 1:1 で対応する。

- **PK:** `player_id` (UUID)
- **UNIQUE INDEX:** `idx_players_firebase_uid` ON `firebase_uid`
- **TRIGGER:** `updated_at` 自動更新

<!-- BEGIN GENERATED: players -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | UUID |
| `firebase_uid` | VARCHAR(128) | No | Firebase Auth UID (Unique) |
| `username` | VARCHAR(50) | No | 表示名 |
| `level` | BIGINT | No | レベル (Default: 1) |
| `exp` | BIGINT | No | 経験値 (Default: 0) |
| `is_premium` | BOOLEAN | No | 課金ステータス |
| `equipped_icon_no` | BIGINT | Yes | 装備中アイコン番号（NULL: デフォルト） |
| `selected_faction` | VARCHAR(20) | Yes | 選択済みファクション |
| `premium_expires_at` | TIMESTAMPTZ | Yes | サブスク有効期限 |
| `created_at` | TIMESTAMPTZ | No | 作成日時 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: players -->

**設計判断:**
- `is_premium` と `premium_expires_at` を players に持たせているのは、ほぼ全 API レスポンスで課金ステータスを返す必要があるため。shop が authoritative だが、`premium-updated` Pub/Sub イベントで account 側に射影を保持する
- `selected_faction` は初回選択時に設定。追加購入した陣営は `player_factions` で管理する

### player_daily_battle

デイリーバトル回数の管理。players と 1:1。

- **PK:** `player_id` (players FK, CASCADE)

<!-- BEGIN GENERATED: player_daily_battle -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `daily_battle_count` | BIGINT | No | 本日のバトル回数 |
| `last_reset_date` | DATE | No | 最終リセット日 |
<!-- END GENERATED: player_daily_battle -->

**設計判断:**
- players テーブルに埋め込まず別テーブルにしているのは、バトル回数チェックは高頻度で呼ばれるが players 本体の更新とは独立しているため。更新競合を避ける

### player_factions

プレイヤーが所持している陣営カードセットの中間テーブル。

- **PK:** `(player_id, faction)`
- **FK:** `player_id` → `players` (CASCADE)
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
- `players.selected_faction` は「現在の選択陣営」を保持するのに対し、`player_factions` は「所持している全陣営」を管理する。ストーリーのアンロック条件判定はこのテーブルを参照する
- `faction-selected` Pub/Sub イベント受信時に INSERT される

### user_settings

ユーザー設定。players と 1:1。

- **PK:** `player_id` (players FK, CASCADE)
- **TRIGGER:** `updated_at` 自動更新

<!-- BEGIN GENERATED: user_settings -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | ユーザーID |
| `language` | VARCHAR(10) | No | 言語設定 |
| `bgm_volume` | BIGINT | No | BGM音量 (0-100) |
| `se_volume` | BIGINT | No | SE音量 (0-100) |
| `push_enabled` | BOOLEAN | No | 通知許可 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: user_settings -->

### processed_events

Pub/Sub subscriber の冪等性を保証するテーブル。

- **PK:** `event_id` (UUID)

<!-- BEGIN GENERATED: processed_events -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `event_id` | UUID | No | Pub/Sub EventID (publisher 生成の UUIDv4) |
| `event_type` | TEXT | No | イベント種別 (faction_selected / premium_updated) |
| `processed_at` | TIMESTAMPTZ | No | 処理日時 |
<!-- END GENERATED: processed_events -->

**設計判断:**
- Pub/Sub の At-Least-Once 配信による重複配送を DB レベルで排除する。INSERT が重複した場合は PK 違反で検知する
- account は `faction-selected` と `premium-updated` の 2 つのイベントを subscribe する

---

## テーブル間リレーション

```
players (PK: player_id)
  │
  ├── 1:1 ── player_daily_battle (FK: player_id, CASCADE)
  ├── 1:N ── player_factions     (FK: player_id, CASCADE)
  └── 1:1 ── user_settings       (FK: player_id, CASCADE)

processed_events (独立、FK なし)
```

---

## インデックス戦略

| インデックス | 対象 | 用途 |
|---|---|---|
| `idx_players_firebase_uid` (UNIQUE) | `players(firebase_uid)` | ログイン時の Firebase UID → player_id lookup。認証フローの起点 |
