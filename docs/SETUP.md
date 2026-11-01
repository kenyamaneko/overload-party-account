# セットアップ

## ローカル開発

`make run` はアプリ本体とインフラ (Postgres / Firestore エミュレータ) を compose 内で起動する。
インフラはホストへ公開せず内部ネットワークのサービス名 DNS で参照するため、他リポのローカル
スタックやホスト上の他アプリとポートが衝突しない。ホストへ出るのは account の API ポート 9005 のみ。

```bash
make run              # アプリ + インフラを compose で起動（ソースをバインドマウント）
make down             # 停止してボリュームを削除
make test             # Testcontainers でテスト実行（Docker 必須）
make test-integration # integration タグ付きテスト（Firestore エミュレータが必要）
make check-schema-factions # db/schema.sql の陣営の CHECK 制約が最新かを確かめる（書き換えない）
```

アプリはコンテナ内で `go run` する。ソースを編集して `docker compose restart account` すれば、
イメージを作り直さずに反映される。プライベートモジュールはホストのモジュールキャッシュを読み取り専用でマウント
して解決するため、`make run` は先にホスト側で `go mod download` を実行する。

game_config の初期データは、common の SSoT の yaml を ops の初期データ投入ツールで流し込むため、
overload-party-ops と overload-party-common を同じ階層に配置しておく必要がある。

## 環境変数

`CLOUDSQL_CONNECTION_NAME` 以外はすべて必須。未設定・不正値は起動時に即失敗する（[internal/config/config.go](../internal/config/config.go) が SSoT。既定値では補わない）。

シークレットとして扱う環境変数:

| 変数名 | 説明 |
|---|---|
| `DATABASE_CONN` | PostgreSQL 接続文字列（pgx が解釈できる URL / libpq 形式） |
| `INTERNAL_AUTH_PUBLIC_KEY` | gateway が発行する RS256 JWT を検証する公開鍵（PEM 形式の RSA 公開鍵）。形式が不正なときは、データベースへ接続する前に起動が失敗する |

その他の環境変数:

| 変数名 | 説明 |
|---|---|
| `PORT` | HTTP リッスンポート（1-65535） |
| `GOOGLE_CLOUD_PROJECT_ID` | Firestore (`game_config`) で利用する Google Cloud プロジェクト ID |
| `LOG_MODE` | `production`（Cloud Logging 互換 JSON）/ `local`（TextHandler）。環境変数の読み込みに失敗したときの出力は、環境変数を読む前に `production` と同じ JSON に設定するため、`LOG_MODE` によらず JSON になる |
| `DATABASE_IAM_AUTH_ENABLED` | `true`（Cloud SQL Go Connector 経由の IAM 認証）/ `false`（`DATABASE_CONN` によるパスワード接続）|
| `CLOUDSQL_CONNECTION_NAME` | Cloud SQL インスタンスの接続名（`project:region:instance`）。`DATABASE_IAM_AUTH_ENABLED=true` のときのみ必須。`false` のときは設定されていても無視する |

ローカルで Firestore エミュレータに接続する場合は `FIRESTORE_EMULATOR_HOST` を併せて設定する（`make run` の compose 定義がエミュレータのサービス名を渡す）。

## Pub/Sub の購読

Pub/Sub の 5 購読は Cloud Run の push 購読経由で `/internal/v1/pubsub/<購読名>` が受ける。購読名は `faction-acquired`・`premium-updated`・`player-onboarded`・`onboarding-name-set`・`onboarding-faction-set` である。到達制御は Cloud Run の呼び出し IAM が担い、受け口自体はアプリ層の認証を持たない。ローカルでは IAM が挟まらないため、`curl` で push 購読と同じ形の本文を直接送って動作確認できる。

受け口の応答は次のとおり。

- **200**：イベントを反映した、または反映済み・反映不要として受理した
- **400**：push の本文を読めない
- **500**：イベントの本文を読めない、または反映に失敗した。Pub/Sub が再配信する

段階を飛ばして進むオンボーディングのイベントは 500 になる。前の段階のイベントが先に反映されたあとの再配信で成功する。
