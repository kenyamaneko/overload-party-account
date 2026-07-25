# Account サービス設計

本ドキュメントは **コードを読んでも一見しては分からない設計意図** だけを残す。実装詳細（エンドポイントごとのバリデーション順・SQL 文・エラー → HTTP ステータス変換・環境変数の一覧）は各ファイルの実装とコメントを一次情報とする。

サービス概要・起動手順は [../README.md](../README.md)、エンドポイントは [../data/openapi.yaml](../data/openapi.yaml) (SSoT)、DB スキーマは [DATA_DESIGN.md](DATA_DESIGN.md)、ビジネス仕様は [FEATURE_SPEC.md](FEATURE_SPEC.md) を参照。

## account サービスの責務境界

account は **プレイヤーの素性（players）と周辺属性（設定・ファクション所有・経験値・デイリーバトル）** の single source of truth。他サービスが必要とする情報は REST 経由でのみ公開する。

### スキーマ所有 (SSoT と read model)

| ドメイン | SSoT | account 側の扱い |
|---|---|---|
| プレイヤー本体 | `account.players` (account) | authoritative |
| レベル / 経験値 | `account.player_progression` (account) | authoritative。1:1 子テーブルとして分離 |
| プレイヤー設定 | `account.player_settings` (account) | authoritative |
| デイリーバトル回数 | `account.player_daily_battle` (account) | authoritative |
| ファクション所有 | `account.player_factions` (account) | authoritative（shop 側でも購入判定用の read model を保持しているが、その構造は shop 側設計を参照） |
| プレミアム状態 | `shop.subscriptions` (shop) | `players.is_premium` / `premium_expires_at` は account 側の射影 |
| ゲームバランス定数 | Cloud Firestore `game_config` | read-only。起動時・リクエスト時に参照 |
| ファクションマスター | `common/data/factions.yaml` | code-generate された定数を参照 |

`player_progression` は `players` から切り出した 1:1 子テーブル。`level` / `exp` は戦闘ごとに高頻度更新されるため、プロフィール系の低頻度更新と物理分離する（「`player_progression` の物理分離」で詳述）。Go の repository 層では `players` + `player_progression` を JOIN で結合し、呼び出し元には「Player アグリゲート」として見せる。

プレミアム状態の authoritative は shop 側のサブスクリプション契約。account は `premium-updated` イベントの subscriber として **最終的整合 (eventually consistent) な射影** を保持する。`is_premium` を参照するほぼ全ての REST レスポンスでローカル SELECT の安定性が必要なため、JOIN ではなく射影として持っている。

repository の interface は **書き込み + 単表 SELECT を扱う `PlayerRepo`** と、**JOIN を伴う表示用 Read Model を返す `PlayerViewRepo`** に分けている。`PlayerView` は `players` + `player_progression` + `player_factions` を JOIN で結合した API レスポンス組み立て用ビュー。これは厳密な CQRS (Read 系は全部 View 側に寄せる) ではなく、**JOIN を含む高コストクエリだけを書き込み Repo から物理的に隔離する** ための分割。`FindByID` のような単表 SELECT は `PlayerRepo` 側に残しており、Write モデルへのロード経路として使う。

### 他サービス呼び出しの禁則

account は他サービスを直接呼び出さない（gateway / shop / scenario / card / battle 含めて）。

- 状態の取り込みは **Pub/Sub subscribe で片方向**
- account から外部への副作用は **REST レスポンスを返すのみ**（Pub/Sub publish しない）

この非対称性は ADR-011 / ADR-014 の schema ownership 契約から来ている。account は「呼ばれるだけ」のサービスに徹するため、外部 API 呼び出しの retry / circuit breaker が実装コードに登場しない。

### `player_progression` の物理分離

`level` / `exp` は `players` に並置せず、1:1 子テーブル `account.player_progression` に切り出している。動機はライフサイクルの違い:

| 概念 | 更新頻度 | 更新者 |
|---|---|---|
| `players` のプロフィール系 (name / is_premium 等) | 低 | ユーザー操作 or 外部イベント |
| `player_progression` (level / exp) | 高 | 戦闘終了ごとに毎回 |

同居させていた従来実装では以下の問題があった:

- `players.updated_at` がバトル頻度で動き、「プロフィール変更の検知」用途に使えない
- 経験値加算の SELECT FOR UPDATE が `players` 全体をロックし、プロフィール更新と競合
- 高頻度 UPDATE の dead tuple が `players` に溜まって VACUUM コスト増

分離後は repository 層が JOIN で Player アグリゲートを組み立てるため、API 契約（レスポンスに level/exp を含む）は維持されている。書き込み側のホットパス（`AddExp`）は `player_progression` のみを触る。

## 認証信頼境界

account は Firebase Auth の ID Token を検証しない。**認証は gateway で完結している前提** で、gateway が検証済みの `firebase_uid` をリクエストボディ / パスパラメータとして、解決済みの player_id を `X-Internal-Auth` (HS256 JWT) の sub クレームとして forward する。

この信頼境界は 3 つの構造で支えている:

1. **ネットワーク**: account は ClusterIP Service のみで公開し、Ingress に乗らない。外部からの到達経路は gateway 経由のみ
2. **ルーター**: [internal/router/router.go](../internal/router/router.go) は bootstrap 系・サーバー間バッチを `/internal/v1/...` に JWT なしで生やし、player-scoped な `/api/v1/account/me/...` には `VerifyInternalAuth`（`X-Internal-Auth` の HS256 JWT 検証）だけを挟む。Firebase ID Token を検証するミドルウェアは存在しない
3. **契約**: `/auth/register` / `/auth/login` は gateway が ID Token を検証して抽出した `firebase_uid` を受け取る。account 側では文字列として扱うだけ

新しいエンドポイントを追加するときも、Firebase ID Token の検証を account 側に導入しないこと。導入した瞬間に「account の中で認証する責務」が混入し、gateway との二重管理になる。

## Register フローのトランザクション設計

`POST /internal/v1/auth/register` は 3 テーブルを同一トランザクションで初期化する:

```
BEGIN TX
  INSERT INTO account.players             (新規 UUID)
  INSERT INTO account.player_progression  (player_id, level=1, exp=0)
  INSERT INTO account.player_settings     (player_id, デフォルト値)
COMMIT
```

`player_daily_battle` は Register では作らない。1 行/プレイヤー/ゲーム日の履歴台帳で、初回バトル時に `IncrementDailyBattleCount` の UPSERT で発生する。当日の行が無ければカウント 0 とみなすため、初期行は不要。

Register は冪等ではない。重複登録は UNIQUE INDEX `idx_players_firebase_uid` と handler 層の `ErrPlayerAlreadyRegistered` → 409 マッピングで吸収する。gateway 側で冪等リトライしたい場合は Login にフォールバックする契約。

### なぜスターターカードや初期ファクションを含めないか

かつて `usecase.AuthInteractor.Register` でスターターカード配布と初期ファクション選択を同時に行っていたが、以下の理由で削除済み:

- ファクション選択は UX 上「チュートリアル開始直後の選択画面」という非同期な操作で、登録と同時に決められない
- カード配布は card サービスの責務で、account が card の内部状態を知るべきでない

トリガーポイントは scenario の各オンボードステップで発行される 3 つのイベント (`onboarding-name-set` / `onboarding-faction-set` / `player-onboarded`) で、account は subscriber 経由で `players.name` / `player_factions (is_initial=TRUE 行)` / `onboarding_status` を反映する（「onboarding-name-set subscriber」「onboarding-faction-set subscriber」「player-onboarded subscriber」の各節）。`usecase.AuthInteractor.Register` にカード付与・ファクション付与を再導入してはいけない（CLAUDE.md 禁止事項）。

## デイリーバトル制限

デイリーバトル制限は `account.player_daily_battle` テーブル (1 行/プレイヤー/ゲーム日の履歴台帳) で管理する。当日の行が無ければカウント 0 とみなす。

### ゲーム日の境界 (JST 05:00)

ゲーム日は **JST 05:00** にリセットする。UTC に +4 時間のオフセットを加算して日付部分を取り出す:

| 実時刻 | ゲーム日 |
|---|---|
| JST 2024-01-02 04:59 (UTC 2024-01-01 19:59) | 2024-01-01 |
| JST 2024-01-02 05:00 (UTC 2024-01-01 20:00) | 2024-01-02 |

実装: `usecase.gameDay()` が `time.Now().UTC().Add(4h)` の日付部分を返す。`gameDayOffset` 定数で一箇所に閉じている ([internal/usecase/player.go](../internal/usecase/player.go))。リセットはアプリ側のロジックではなく、PK `(player_id, game_date)` が日ごとに別行を区別することで自動的に行われる。

### 履歴台帳としての設計

1 行/プレイヤー/ゲーム日で過去のバトル回数を保持する。これはアプリ内でプレイヤーごとのバトル回数履歴が残る唯一の場所であり、将来 BigQuery エクスポートでプレイヤーごとの日次推移を分析できるようにするための土台。Register 時に当日の初期行を作らず、初回バトルの UPSERT で発生させる (登録と日次台帳のライフサイクルを切り離す目的)。

### 上限ガードと TOCTOU の判断

上限判定は usecase 層 (`PlayerInteractor`) の責務で、repository 層 (`PlayerRepository.IncrementDailyBattleCount`) は `(player_id, game_date)` を 1 SQL で UPSERT し加算後のカウントを返すプリミティブ。`free_daily_battle_limit` が Firestore 未設定 (値 0) のときはフォールバックせずエラーを返す (運用事故として扱う)。

account が `player_daily_battle` の authoritative owner なので、上限不変条件は account 側で守る。battle サービス側が事前に `GetBattleLimit` を呼ぶのは UX のためのプリチェック (「戦う前に残り回数を表示」) であり、最終的な強制は account の書き込みパスで行う二段構え。

TOCTOU は、free プレイヤーの上限超過判定で `GetDailyBattle` → UPSERT の隙間が原理的には残る (短時間に上限ぎりぎりの並行バトルがあれば +1 通る可能性) が、同一アカウントの並行バトルは極めて稀なエッジケースとして許容する。行ロック (`SELECT FOR UPDATE`) は採用しない。インクリメント自体は単発の UPSERT で原子的なので、カウントが飛んだり重複したりはしない。

## プレイヤー設定の部分更新契約

`PUT /api/v1/account/me/settings` は HTTP メソッドこそ PUT だが、**部分更新セマンティクス** を採用する (REST 慣用的には PATCH が正しいが、呼び出し元への影響を避けるため PUT で運用)。

「クライアントが language だけ変えるつもりで部分送信したら、送信しなかった bgm_volume がゼロ値で上書きされる」事故を避けるための設計。repo 層では `Insert` (Register 用、全フィールド必須) と `UpdatePartial` (更新用、nil は現状維持) の 2 プリミティブに分離し、Upsert のような「書いてあることを丸ごと反映」パターンは排除している。具体的なフィールド型・SQL は実装を参照。

## Pub/Sub subscriber

account は 5 つの Pub/Sub push subscription を `/internal/v1/pubsub/<イベント名>` の受け口で受ける。push subscription は at-least-once 配信のみで exactly-once 配信をサポートしないため、**`processed_events` による冪等ガードが唯一の防御**になる（再配信は例外ではなく通常の挙動として扱う）。

到達制御は Cloud Run の呼び出し IAM が担い、受け口自体はアプリ層の認証を持たない。push envelope のデコード・応答コードへの変換は [internal/handler/pubsubpush](../internal/handler/pubsubpush) が共通化し、イベントごとの処理は各 subscriber の `HandleMessage` に委譲する。

### faction-acquired subscriber

受け口: `POST /internal/v1/pubsub/faction-acquired`

発行元: shop のみ。shop 購入時のみ発火する単一業務事実イベント（ADR-022 / ADR-031）。

| publisher | 副作用 |
|---|---|
| shop | `player_factions` INSERT のみ (`is_initial=FALSE` 固定) |

処理（[internal/adapter/pubsub/faction_acquired_subscriber.go](../internal/adapter/pubsub/faction_acquired_subscriber.go)）:

```
BEGIN TX
  INSERT processed_events (event_id, event_type)  ← ON CONFLICT DO NOTHING
  IF 既存行だった: COMMIT; 成功として return
  INSERT player_factions (player_id, faction, is_initial=FALSE)
    ON CONFLICT (player_id, faction) DO NOTHING
COMMIT → 成功
```

scenario 起因の初期 faction 確定 (かつて `faction-selected(source=scenario_initial)` が担っていた) は onboarding-faction-set subscriber に統合された。

### premium-updated subscriber

受け口: `POST /internal/v1/pubsub/premium-updated`

発行元: shop のみ。shop がサブスクリプション状態遷移（開始 / 更新 / 期限切れ / 失効）のうち **premium が変化する遷移で publish する**（cancel 時は publish しない契約、shop 側 ARCHITECTURE.md 参照）。

処理:

```
BEGIN TX
  INSERT processed_events ...  ← 冪等ガード
  UPDATE players SET is_premium=$, premium_expires_at=$ WHERE player_id=$
COMMIT → 成功
```

### onboarding-name-set subscriber

受け口: `POST /internal/v1/pubsub/onboarding-name-set`

発行元: scenario。オンボード内 name 入力ステップで scenario が account の validate REST 成功後に publish する。

処理（[internal/adapter/pubsub/onboarding_name_set_subscriber.go](../internal/adapter/pubsub/onboarding_name_set_subscriber.go)）:

```
BEGIN TX
  INSERT processed_events (event_id, event_type)  ← ON CONFLICT DO NOTHING
  IF 既存行だった: COMMIT; 成功として return
  UPDATE players SET name = $
  onboarding_status を 'name_set' へ前進 (後進遷移はスキップ)
COMMIT → 成功
```

`onboarding_status` の遷移は `domain.CanTransitionOnboardingStatus` で前進方向のみ許容する。
out-of-order 配信で先に completed が反映済みでも整合性が保たれる。

### onboarding-faction-set subscriber

受け口: `POST /internal/v1/pubsub/onboarding-faction-set`

発行元: scenario。オンボード内 faction 選択ステップで scenario の `SelectableFactions` 検証成功後に publish する。

処理（[internal/adapter/pubsub/onboarding_faction_set_subscriber.go](../internal/adapter/pubsub/onboarding_faction_set_subscriber.go)）:

```
BEGIN TX
  INSERT processed_events (event_id, event_type)  ← ON CONFLICT DO NOTHING
  IF 既存行だった: COMMIT; 成功として return
  onboarding_status を 'faction_set' へ前進 (後進遷移はスキップ)
  INSERT player_factions (player_id, faction, is_initial=TRUE)
    ON CONFLICT (player_id, faction) DO UPDATE SET is_initial = TRUE
COMMIT → 成功
```

### player-onboarded subscriber

受け口: `POST /internal/v1/pubsub/player-onboarded`

発行元: scenario。`POST /onboarding/complete` 受領時に transactional outbox 経由で publish する。

処理（[internal/adapter/pubsub/player_onboarded_subscriber.go](../internal/adapter/pubsub/player_onboarded_subscriber.go)）:

```
BEGIN TX
  INSERT processed_events (event_id, event_type)  ← ON CONFLICT DO NOTHING
  IF 既存行だった: COMMIT; 成功として return
  UPDATE players SET onboarding_status = 'completed'
COMMIT → 成功
```

initial faction の永続化は onboarding-faction-set subscriber が先行している前提。本 subscriber の責務は `completed` への状態遷移のみ。

### `processed_events` による冪等性契約

- 1 行 = (`event_id`, `event_type`, `processed_at`) の 3 カラム。`event_id` が PK
- subscriber はトランザクション冒頭で `INSERT ... ON CONFLICT DO NOTHING RETURNING event_id`
- `RETURNING` が空 → 既に処理済み → トランザクション内の後続処理をスキップして成功として応答する
- 処理本体と `processed_events` INSERT を同一トランザクションに揃えているため、部分適用は発生しない

### エラー時の応答コードと DLQ

| 失敗種別 | 応答 |
|---|---|
| push envelope の形が不正 (JSON 構造 / message.data 欠落) または base64 復号失敗 | 400（[internal/handler/pubsubpush](../internal/handler/pubsubpush) が envelope を受理できず subscriber に到達しない） |
| JSON デシリアライズ失敗 (event 本体) | 500（Pub/Sub 側で再配信 → 最終的に DLQ へ） |
| `event_type` が未知 | 200（握りつぶしではなく「この subscriber の責務外」として意図的にスキップ。ログは残す） |
| DB エラー / トランザクション失敗 | 500（一時的障害としてリトライさせる） |

未知の `event_type` を 200 にするのは、将来 publisher 側で新しいイベント種別を追加した際に account の subscriber を止めないため。既知の event_type のペイロードが壊れているケースは JSON デシリアライズ失敗側に分岐する。

`ApplyFactionSet` は `processed_events` を Insert した後に `ErrFactionConflict`（同一プレイヤーに別 faction で再到着）を検出すると Tx 全体をロールバックする。`processed_events` 行も巻き戻るため再配信のたびに同じ衝突が再発し、最終的に DLQ へ流れる挙動になる。これは publisher 側のバグ（同一プレイヤーへの矛盾する faction-set publish）でしか起きない想定で、自動リカバリせず DLQ で滞留させて運用に検知させる設計。救出は publisher 側の不整合を是正したうえで、DLQ メッセージを破棄するか正しいペイロードに差し替えて replay する。

## 経験値・レベル計算: 係数の SSoT は Firestore

経験値獲得量 (`exp_win` / `exp_loss` / `exp_draw`) とレベル計算係数 (`exp_formula_coefficient`) は Firestore `game_config` から読む。DB には置かない。

理由: ゲームバランス調整は運用者が Firestore コンソールからコードデプロイなしで変えるため。account 側はハードコードしない。

未設定（値 0 または存在しない）は起動エラーにせず、当該リクエストをエラーにする（運用者が値を戻すまで battle 側で経験値が積めない）。

## ドメイン層の責務 (データ struct と振る舞いの分離)

`internal/domain` パッケージは 2 種類のファイルで構成される:

| 種類 | ファイル例 | 役割 |
|---|---|---|
| 手書き struct | `player.go` | テーブル 1 行に対応する持ち回り型のみを保持し、メソッドは持たない。db タグだけ付与 |
| 手書きの振る舞い | `name.go` / `onboarding_status.go` / `defaults.go` | バリデーション・遷移ルール・不変条件・既定値などのドメインロジック。同パッケージなので import なしで自然に組み合わさる |

### 振る舞いを domain に置く基準

「Anemic か Rich か」の二者択一ではなく、**昇格基準**で判断する:

- **昇格させる**: 複数 usecase で同じ判定ロジックを書きそうになった / struct のフィールドを直接読むだけでは業務上の意図が掴めない / 不正値を構築できないようにする不変条件がある
- **昇格させない**: 1 箇所の usecase でしか使わない短絡的な判定 (例: 現状の `if player.IsPremium`) / 永続化や外部呼び出しを含む処理 (= usecase の責務)

将来必要になりそうという理由だけで先回りしてメソッドを足さない。重複や複雑さが顕在化してから手書きファイルに昇格させる。

## エラーハンドリングのレイヤ責務

| 層 | 返す/扱う |
|---|---|
| repository | `port.ErrNotFound` と wrap された SQL エラー |
| usecase | ドメインのセンチネル ([internal/usecase/errors.go](../internal/usecase/errors.go)) + wrap された下位エラー |
| handler (rest) | `errors.Is` でセンチネルを分類し HTTP ステータスに変換 ([internal/handler/rest/errors.go](../internal/handler/rest/errors.go)) |

usecase 層は HTTP ステータスを知らず、handler 層は SQL を知らない。センチネルと HTTP ステータスのマッピングは `errors.go` を SSoT とし、各センチネルの docstring に「なぜ 409 か」「冪等な成功扱いかどうか」等のセマンティクスを書く。

## 運用

### 環境変数 / Secret Manager

環境変数の一覧と必須条件は [internal/config/config.go](../internal/config/config.go) の `FromEnv` が SSoT（欠ければ即 fail）。運用上の注意点のみ:

- `DATABASE_CONN` は Secret Manager 由来。k8s マニフェストにインラインしない
- `GOOGLE_CLOUD_PROJECT_ID` は ConfigMap 経由で環境ごとに切り替え (Firestore で使用)
- Pub/Sub の push subscription 名・エンドポイント URL は Terraform 側が管理し、account のコードは関知しない ([overload-party-infra](https://github.com/kenyamaneko/overload-party-infra) の担当)

### Pub/Sub トピックと subscriber

| トピック | 発行元 | account の受け口 | account 側の副作用 |
|---|---|---|---|
| `faction-acquired` | shop | `POST /internal/v1/pubsub/faction-acquired` | `player_factions` INSERT (`is_initial=FALSE`、「faction-acquired subscriber」) |
| `premium-updated` | shop | `POST /internal/v1/pubsub/premium-updated` | `players.is_premium` / `premium_expires_at` UPDATE (「premium-updated subscriber」) |
| `onboarding-name-set` | scenario | `POST /internal/v1/pubsub/onboarding-name-set` | `players.name` UPDATE + `onboarding_status='name_set'` を 1 tx で実行 (「onboarding-name-set subscriber」) |
| `onboarding-faction-set` | scenario | `POST /internal/v1/pubsub/onboarding-faction-set` | `player_factions` UPSERT (`is_initial=TRUE`) + `onboarding_status='faction_set'` を 1 tx で実行 (「onboarding-faction-set subscriber」) |
| `player-onboarded` | scenario | `POST /internal/v1/pubsub/player-onboarded` | `players.onboarding_status='completed'` UPDATE のみ (「player-onboarded subscriber」) |

account 自身はトピックを publish しない。

### Firestore の運用

`game_config` コレクションは運用者が手動で値を書く（コード上には生成スクリプトを持たない）。キーのリストと意味は [FEATURE_SPEC.md](FEATURE_SPEC.md) と `usecase/player.go` の定数を参照。
