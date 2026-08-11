# overload-party-account

プレイヤーマスター・ユーザー設定・ファクション所有・経験値・デイリーバトル制限を所有する内部マイクロサービス。ポート 9005 で起動する。

詳細は [API契約 (OpenAPI)](data/openapi.yaml) / [データ設計書](docs/DATA_DESIGN.md) を参照。設計判断 (Why) は [common の ADR](https://github.com/kenyamaneko/overload-party-common/tree/main/docs/adr) に記録する。

[テスト観点カタログ](https://kenyamaneko.github.io/overload-party-account/): テスト名から生成した、テスト済みの観点の一覧。

## アーキテクチャ概要

```
Gateway (唯一の入口)
  └─ Account (:9005)                 到達は gateway からのみ / 認証は gateway 側で完了済み
       ├─ PostgreSQL (account スキーマ)
       ├─ Cloud Firestore (game_config 読み取り専用)
       └─ Pub/Sub subscriber
            ├─ faction-acquired-account-sub        ← shop が publish
            ├─ premium-updated-account-sub         ← shop が publish
            ├─ onboarding-name-set-account-sub     ← scenario が publish
            ├─ onboarding-faction-set-account-sub  ← scenario が publish
            └─ player-onboarded-account-sub        ← scenario が publish

Gateway / shop / scenario → Account (player-scoped, JWT 必須)
  └─ /api/v1/account/me/...                X-Internal-Auth (RS256 JWT) を検証し sub で player_id 解決

Battle → Account (server-to-server, JWT なし)
  └─ POST /internal/v1/players/award-game-exp
```

account は他サービスを直接呼び出さない。状態の取り込みはすべて Pub/Sub subscribe で
片方向に行い、`processed_events` テーブルでアプリ層の冪等性を担保する。

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
イメージを作り直さずに反映される。private module は host の module cache を読み取り専用でマウント
して解決するため、`make run` は先に host 側で `go mod download` を実行する。

game_config の seed は common の SSoT yaml を ops の seed ツールで流し込むため、
overload-party-ops / overload-party-common を兄弟ディレクトリに checkout しておく必要がある。

## 環境変数

すべて必須。未設定・不正値は起動時に即 fail する（[internal/config/config.go](internal/config/config.go) が SSoT、デフォルトへのフォールバック禁止）。

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

## 公開パッケージ

[packages/api-account/](packages/api-account/) に REST 契約型（`apiaccount.PlayerResponse` 等）を公開している。
[data/openapi.yaml](data/openapi.yaml) が SSoT。編集後に以下で `openapi_gen.go` を再生成する。

```bash
scripts/generate_types.sh
```

クライアント向け TypeScript 型は `@kenyamaneko/overload-party-api-gateway` に統合済み。
