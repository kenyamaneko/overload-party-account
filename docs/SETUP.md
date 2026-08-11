# セットアップ

## ローカル開発

`make run` はアプリ本体とインフラ (Postgres / Firestore / Pub/Sub emulator) を compose 内で起動する。
インフラはホストへ publish せず内部ネットワークのサービス名 DNS で参照するため、他リポのローカル
スタックやホスト上の他アプリとポートが衝突しない。ホストへ出るのは account の API ポート 9005 のみ。

```bash
make run              # アプリ + インフラを compose で起動（ソース bind-mount）
make down             # 停止して volume を削除
make test             # Testcontainers でテスト実行（Docker 必須）
make test-integration # integration タグ付きテスト（Pub/Sub emulator などを要するもの）
```

アプリはコンテナ内で `go run` する。ソースを編集して `docker compose restart account` すれば、
イメージを作り直さずに反映される。private module はホストの module cache を読み取り専用でマウント
して解決するため、`make run` は先にホスト側で `go mod download` を実行する。

game_config の seed は common の SSoT yaml を ops の seed ツールで流し込むため、
overload-party-ops と overload-party-common を同じ階層に配置しておく必要がある。

## 環境変数

すべて必須。未設定・不正値は起動時に即失敗する（[internal/config/config.go](../internal/config/config.go) が SSoT、デフォルトへのフォールバック禁止）。

シークレットとして扱う環境変数:

| 変数名 | 説明 |
|---|---|
| `DATABASE_CONN` | PostgreSQL 接続文字列（pgx が解釈できる URL / libpq 形式） |
| `INTERNAL_AUTH_PUBLIC_KEY` | gateway が発行する RS256 JWT を検証する公開鍵（PEM） |

その他の環境変数:

| 変数名 | 説明 |
|---|---|
| `PORT` | HTTP リッスンポート（1-65535） |
| `GOOGLE_CLOUD_PROJECT_ID` | Firestore (`game_config`) で利用する Google Cloud プロジェクト ID |
| `LOG_MODE` | `production`（Cloud Logging 互換 JSON）/ `local`（TextHandler）|
| `DATABASE_IAM_AUTH_ENABLED` | `true`（Cloud SQL Go Connector 経由の IAM 認証）/ `false`（`DATABASE_CONN` によるパスワード接続）|
| `CLOUDSQL_CONNECTION_NAME` | Cloud SQL インスタンスの接続名（`project:region:instance`）。`DATABASE_IAM_AUTH_ENABLED=true` のときのみ必須 |

ローカルで Firestore emulator に接続する場合は `FIRESTORE_EMULATOR_HOST` を併せて設定する（`make run` の compose 定義が emulator のサービス名を渡す）。

Pub/Sub の 5 購読は Cloud Run push subscription 経由で `/internal/v1/pubsub/<イベント名>` が受ける。到達制御は Cloud Run の呼び出し IAM が担い、受け口自体はアプリ層の認証を持たない。ローカルでは IAM が挟まらないため、`curl` で直接 push envelope を投げて動作確認できる。
