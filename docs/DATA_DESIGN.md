# account スキーマ - データ設計

> **DDL の SSoT:** `db/schema.sql`
> **テーブル内カラム表の再生成:** `python3 scripts/generate_schema_doc.py` が `<!-- BEGIN/END GENERATED -->` マーカー内を上書きする。マーカー外の設計判断・リレーション図は手動で保守する。
> **陣営の CHECK 制約の再生成:** `go run scripts/generate_schema_factions.go` が `db/schema.sql` の陣営の値を、陣営の SSoT (`common/data/factions.yaml`) から生成した定数に合わせて書き換える。`-check` を付けると書き換えず、最新かどうかだけを確かめる (`make check-schema-factions`)。

## 設計概要

account スキーマはプレイヤーの基本情報・ゲーム日ごとのバトル回数・陣営の所持・プレイヤー設定・冪等性のための記録 (Pub/Sub 購読用と REST 呼び出し用) を所有する。全サービスの中で最も多くの外部参照を受けるスキーマだが、他サービスからの直接 SELECT は許容せず account の内部 REST API 経由でのみ公開する（ADR-011 / ADR-014）。

`is_premium` / `premium_expires_at` は shop が正本として持つ状態を写しているだけで、account 内では参照専用の扱い（書き込みは `premium-updated` 購読と、運用・テスト用の `PUT /api/v1/account/me/premium` のみ）。

ゲームバランス調整値 (1 日の無料バトル回数の上限・経験値係数など) は account スキーマに持たず、Cloud Firestore `game_config` を読み取り専用で参照する（[ADR-017](https://github.com/kenyamaneko/overload-party-common/blob/main/docs/adr/017-game-config-firestore.md)）。陣営のマスターデータも DB に持たず、`common/data/factions.yaml` から生成された定数を参照する。

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
| `firebase_uid` | VARCHAR(128) | No | Firebase Auth UID (一意) |
| `name` | VARCHAR(50) | Yes | 表示名 (NULL: 未設定) |
| `is_premium` | BOOLEAN | No | プレミアム状態 |
| `equipped_icon_no` | BIGINT | Yes | 装備中アイコン番号（NULL: デフォルト） |
| `onboarding_status` | VARCHAR(20) | No | オンボーディング進行状態 |
| `premium_expires_at` | TIMESTAMPTZ | Yes | プレミアムの有効期限 |
| `last_premium_event_occurred_at` | TIMESTAMPTZ | Yes | 最後に反映したプレミアム状態更新イベントの発生時刻 (NULL: 未反映) |
| `created_at` | TIMESTAMPTZ | No | 作成日時 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: players -->

**設計判断:**
- `is_premium` / `premium_expires_at` を players に写して持つ理由は、ほぼ全ての REST レスポンスでプレミアム状態を返す必要があり、毎回 shop を呼ぶと結合が強くなりすぎるため。shop が正本で、`premium-updated` の Pub/Sub イベントで結果整合性を保つ
- `last_premium_event_occurred_at` は、古いプレミアム状態更新イベントが新しいイベントの結果を上書きしないよう、最後に反映したイベントの発生時刻 (イベント本文の `timestamp`) を保存する。イベントの発生時刻がこの列より古いときは、状態も有効期限も書き換えず、成功として受理する。同じ時刻か新しい時刻のとき、またはこの列が NULL のときは反映する。別のイベントIDのイベントが同時に届いても古いイベントが上書きしないよう、比較と更新は同じプレイヤーの行を排他的に取得 (`SELECT ... FOR UPDATE`) してから行う。この列を書くのはイベントの反映 (`UpdatePremiumFromEvent`) だけで、発生時刻を持たない REST の `PUT /api/v1/account/me/premium` (`UpdatePremium`) は変えない
- 「オンボーディングで選択した陣営」は `players` 側に列を置かず、`player_factions.is_initial=TRUE` の行が SSoT (「player_factions」参照)
- `name` は「表示名」。Register 時には NULL で挿入し、オンボーディング中の `onboarding-name-set` イベントを受けた購読で確定する。account 側で `display_name` 列を別途設けない（同じ意味の列を複数持つと SSoT が分散するため）
- `onboarding_status` は、隣の段階への前進と、同じ状態の指定だけを反映する。段階を飛ばす前進 (例: `not_started` のプレイヤーへの完了イベント) はエラーにし、購読は 500 を返して再配信に回す。到着順が入れ替わっただけで、前の段階のイベントが先に反映されれば再配信で成功するため、発行元の誤りとは区別して扱う。後退 (例: 完了済みのプレイヤーへの表示名設定イベント) は警告ログを残し、表示名も初期陣営も書かずに受理する
- `equipped_icon_no` は shop 側の `cosmetic_items(item_type='icon', item_no=N)` を参照するが、スキーマをまたぐ外部キーは張らない（アプリ層で整合性を保つ）

### player_daily_battle

ゲーム日ごとのバトル回数の履歴台帳。1 行 = 1 プレイヤー × 1 ゲーム日。

- **PK:** `(player_id, game_date)` (`player_id` → `players.player_id`, ON DELETE CASCADE)

<!-- BEGIN GENERATED: player_daily_battle -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `game_date` | DATE | No | ゲーム日 (JST 05:00 で切り替わる) |
| `daily_battle_count` | BIGINT | No | そのゲーム日のバトル回数 |
<!-- END GENERATED: player_daily_battle -->

**設計判断:**
- players に埋め込まず別テーブルにしているのは、バトル回数の確認と加算が高頻度で走るのに対し、players 本体の更新 (name / is_premium 等) とは独立しているため。更新競合を分離する目的
- ゲーム日は JST 05:00 で切り替わる。`game_date` は usecase が時刻から算出する civil.Date
- 1 行/日で履歴を残すのは、プレイヤーごとの 1 日のバトル回数を分析できるようにするため。これがアプリ内で履歴が残る唯一の場所
- バトル回数の返却では、この表の行を排他的に取得 (`SELECT ... FOR UPDATE`) してから減らす (「battle_count_reversals」参照)。減算は 1 減らすだけで、0 を下回らないようにする判断は usecase が持つ
- バトル回数の加算は、1 つのトランザクションの中で、先に当日の行を 1 増やし (UPSERT が行ロックを取る)、無料プレイヤーで増やした後の回数が上限を超えたときはトランザクションを巻き戻す。回数を読んで確かめてから増やす方式では、同時の加算が確認をすり抜けて上限を超えるため。上限との比較は usecase が持ち、SQL に上限の条件を入れない
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
- `processed_events` は Pub/Sub 購読の冪等性専用のため流用せず、REST 呼び出しの冪等性は別テーブルに持つ
- `INSERT ... ON CONFLICT DO NOTHING RETURNING game_id` で重複呼び出しを検知する。`processed_events` と同じパターンだが対象イベントの性質が異なるためテーブルを分ける
- 返却対象の `game_date` は保持しない。呼び出し側が渡す消費時刻から都度算出するため、本テーブルは「同じ対戦を二度処理しない」ことだけを保証する
- 返却は 1 つのトランザクションで、次の順に行う。(1) この表に記録する。記録済みなら何もせず成功する。(2) プレイヤーごとに、消費時刻のゲーム日の `player_daily_battle` の行を排他的に取得する。(3) 行が無いときは警告ログを残して成功する。(4) 回数が 0 のときは何も減らさず成功する。(5) 回数が 1 以上のときだけ 1 減らす。排他的に取得するのは、同じプレイヤーの返却が同時に走っても、回数が 0 を下回らないようにするため
- `game_id` は UUID 型にせず TEXT にしている。account は battle が発行する識別子の具体的なフォーマットを持たないため、フォーマットの前提を account 側の DDL に持ち込まない

### player_factions

プレイヤーが所持している陣営と、オンボーディングで選択した初期陣営を兼ねるテーブル。

- **PK:** `(player_id, faction)`
- **FK:** `player_id` → `players` (ON DELETE CASCADE)
- **CHECK:** `faction` は陣営の SSoT (`common/data/factions.yaml`) にある全陣営の ID のいずれか。値の一覧は `scripts/generate_schema_factions.go` が `db/schema.sql` に書き込む
- **PARTIAL UNIQUE INDEX:** `idx_player_factions_initial` ON `(player_id) WHERE is_initial = TRUE` (1 プレイヤーに初期陣営は最大 1 つ)

<!-- BEGIN GENERATED: player_factions -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `faction` | VARCHAR(20) | No | 陣営名 |
| `is_initial` | BOOLEAN | No | オンボーディングで選択した陣営か (1 プレイヤーにつき最大 1 行 TRUE) |
| `acquired_at` | TIMESTAMPTZ | No | 取得日時 |
<!-- END GENERATED: player_factions -->

**設計判断:**
- 所持リストと「オンボーディングで選択した陣営」を 1 テーブルに集約。`is_initial=TRUE` の行が後者で、部分一意インデックスで「1 プレイヤーに最大 1 つ」を DB レベルで保証する
- ショップ先行で買った行をオンボーディングで初期陣営に確定するときは、同じ行を `is_initial=TRUE` に昇格させる
- 初期陣営は、usecase が未設定を確かめてから設定する。同時に初期陣営が設定されて確認をすり抜けたときは、repository が、設定済みの初期陣営 (同じ陣営でも別の陣営でも) との衝突を重複を示すエラーにして返す。別の陣営は部分一意インデックスの違反で、同じ陣営は設定済みの行を更新しなかったことで検出する。REST の選択は契約の 409 (選択済み) にし、オンボーディングの陣営設定イベントは処理失敗として再配信に回す。再配信では確定済みの初期陣営と比べて、同じ陣営なら成功、別の陣営なら衝突のエラーになる
- 複合 PK `(player_id, faction)` が冪等性のキー
- 陣営のマスターデータの SSoT は `common/data/factions.yaml` から生成された定数のため、`factions` のような参照用のテーブルは持たず、DB 側では CHECK 制約で値を限定する。CHECK 制約の値は、SSoT の変更に追従できるよう手で編集せず、`go run scripts/generate_schema_factions.go` で生成する
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
- 登録時（Register）に `player_settings` 行をアプリ層デフォルトで INSERT する
- 言語は列の上限 (`VARCHAR(10)`) に合わせて 10 文字までとし、REST の受け口が 11 文字以上の言語を 400 にする。DB に渡すと列の長さ制約違反で 500 になり、呼び出し元が次に取れる行動が伝わらないため

### player_progression

累計経験値。players と 1:1 の子テーブル。

- **PK:** `player_id` (→ `players.player_id`, ON DELETE CASCADE)
- **TRIGGER:** `trg_player_progression_updated_at`：UPDATE 時に `updated_at` を自動更新

<!-- BEGIN GENERATED: player_progression -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `player_id` | UUID | No | 親テーブル参照 |
| `exp` | BIGINT | No | 経験値 (初期値: 0) |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: player_progression -->

**設計判断:**
- `exp` を `players` と別の表にしているのは、戦闘ごとの高頻度 UPDATE が `players.updated_at` を押し上げプロフィール変更の検知を汚染する・`players` 全体の SELECT FOR UPDATE で他 UPDATE と競合する・MVCC dead tuple が `players` に集中する、などの問題を避けるため（[ADR-068](https://github.com/kenyamaneko/overload-party-common/blob/main/docs/adr/068-account-player-progression-separate-table.md)）
- API レスポンスの `Player` には `level` / `exp` を含めるため、repository が `players`・`player_progression`・初期陣営を JOIN して参照用の情報 (`domain.PlayerView`) として返す。usecase がこれの累計経験値と経験値係数からレベルを求め、レベルの中での経験値の進み具合を加えて、プレイヤー情報 (`domain.PlayerInfo`) にする
- 経験値の加算 (`POST /api/v1/account/me/exp` と `POST /internal/v1/players/award-game-exp`) は `player_progression` だけを更新し、`players` は更新しない
- 経験値係数の検証は起動時だけで、実行時はリクエストごとに Firestore から読み直した値を使う。運用中に係数を 1 未満へ変えるとレベルの算出が終わらなくなるため、係数が 1 未満のときはレベルの算出と進み具合の算出がエラーになり、REST は 500 を返す。累計経験値が負のときも同じくエラーになり、REST は 500 を返す。
- レベルは累計経験値と経験値係数だけから決まる値のため、DB に保存せず、プレイヤー情報の取得 (ログインを含む) と登録のたびに求める。経験値の加算は累計経験値だけを更新する

### processed_events

Pub/Sub 購読の冪等性を保証する、アプリ層のガードテーブル。5 つの購読すべてが、この表に処理済みのイベントを記録する。

- **PK:** `event_id` (UUID)
- **FK なし**（players とはライフサイクルが独立）

<!-- BEGIN GENERATED: processed_events -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `event_id` | UUID | No | Pub/Sub のイベントID (発行元が生成する UUIDv4) |
| `event_type` | TEXT | No | 受信したイベントの event_type |
| `processed_at` | TIMESTAMPTZ | No | 処理日時 |
<!-- END GENERATED: processed_events -->

**設計判断:**
- account は Pub/Sub の push 購読で受信する。push は at-least-once 配信のみで exactly-once をサポートしないため、本テーブルが冪等性を保証する唯一の防御になる
- usecase が `INSERT ... ON CONFLICT DO NOTHING RETURNING event_id` で重複を検知する。`RETURNING` が空なら処理本体をスキップして成功として応答する
- 処理本体と INSERT は同一トランザクション内で完結するため、「適用されたのに event_id が記録されない」状態は構造的に発生しない。処理本体が失敗したときは INSERT も取り消されるので、再配信された同じイベントIDのイベントを改めて処理できる
- 発生時刻が古いために反映しないプレミアム状態更新イベントと、後退のために反映しないオンボーディングのイベントは、成功として受理するので、イベントIDを記録する
- 段階を飛ばしたオンボーディングのイベントは失敗として扱い、イベントIDを記録しない (「players」の設計判断を参照)

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

[shop.subscriptions] ─ ─ ─ (スキーマをまたぐ、premium-updated 経由のアプリ層の整合)
        └─→ players.is_premium / premium_expires_at  (写し)

[shop.player_owned_factions] ─ ─ ─ (スキーマをまたぐ、読み取り用の写し)
        ←─  player_factions  (正本)
```

点線は、スキーマやサービスをまたぐ、アプリ層の整合を表す。DB の外部キーは張らず、Pub/Sub イベントと購読の冪等性で結果整合性を保つ。

---

## インデックス戦略

| インデックス | 対象 | 用途 |
|---|---|---|
| `idx_players_firebase_uid` (UNIQUE) | `players(firebase_uid)` | ログイン時に Firebase UID から player_id を引く。認証フローの起点 |
| `idx_player_factions_initial` (部分 UNIQUE: `WHERE is_initial = TRUE`) | `player_factions(player_id)` | 1 プレイヤーに初期陣営を最大 1 つにする。初期陣営の取得 (`player_id` と `is_initial = TRUE` で絞る) も、このインデックスで引く |

この 2 つ以外のセカンダリインデックスは定義していない。PK（`player_id` / 複合 PK）が、他の主要クエリパスをカバーするため。
