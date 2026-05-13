# overload-party-account

プレイヤーマスター・ユーザー設定・ファクション所有・経験値・デイリーバトル制限を所有する内部マイクロサービス。ポート 9005 で起動する。

詳細は [機能仕様書](docs/FEATURE_SPEC.md) / [サービス設計書](docs/ARCHITECTURE.md) / [API契約 (OpenAPI)](data/openapi.yaml) / [データ設計書](docs/DATA_DESIGN.md) を参照。

## アーキテクチャ概要

```
Gateway (唯一の入口)
  └─ Account (:9005)                 ClusterIP のみ / 認証は gateway 側で完了済み
       ├─ PostgreSQL (account スキーマ)
       ├─ Cloud Firestore (game_config 読み取り専用)
       └─ Pub/Sub subscriber
            ├─ faction-acquired-account-sub  ← shop が publish
            ├─ premium-updated-account-sub   ← shop が publish
            └─ player-onboarded-account-sub  ← scenario が publish

Gateway / shop / scenario → Account (player-scoped, JWT 必須)
  └─ /api/v1/account/me/...                X-Internal-Auth (HS256 JWT) を検証し sub で player_id 解決

Battle → Account (server-to-server, JWT なし)
  └─ POST /internal/v1/players/award-game-exp
```

account は他サービスを直接呼び出さない。状態の取り込みはすべて Pub/Sub subscribe で
片方向に行い、`processed_events` テーブルでアプリ層の冪等性を担保する。

## ローカル開発

```bash
make db-up            # postgres:16-alpine を起動
make run              # サーバー起動（db-up + env 一式を Makefile が注入）
make test             # Testcontainers でテスト実行（Docker 必須）
make test-integration # integration タグ付きテスト（Pub/Sub emulator などを要するもの）
make db-down          # 停止
make db-reset         # volume ごと削除して再作成
```

## 環境変数

すべて必須。未設定・不正値は起動時に即 fail する（[internal/config/config.go](internal/config/config.go) が SSoT、デフォルトへのフォールバック禁止）。

Secret:

| 変数名 | 説明 |
|---|---|
| `DATABASE_CONN` | PostgreSQL 接続文字列（pgx が解釈できる URL / libpq 形式） |
| `INTERNAL_AUTH_SECRET` | gateway / 各サービスで共有する HS256 JWT 鍵（ADR-037） |

ConfigMap:

| 変数名 | 説明 |
|---|---|
| `PORT` | HTTP リッスンポート（1-65535） |
| `GOOGLE_CLOUD_PROJECT_ID` | Pub/Sub 購読 / Firestore (`game_config`) で利用する Google Cloud プロジェクト ID |
| `FACTION_ACQUIRED_SUBSCRIPTION` | faction-acquired の pull subscription 名 |
| `PREMIUM_UPDATED_SUBSCRIPTION` | premium-updated の pull subscription 名 |
| `PLAYER_ONBOARDED_SUBSCRIPTION` | player-onboarded の pull subscription 名 |
| `ONBOARDING_NAME_SET_SUBSCRIPTION` | onboarding-name-set の pull subscription 名 |
| `ONBOARDING_FACTION_SET_SUBSCRIPTION` | onboarding-faction-set の pull subscription 名 |
| `LOG_MODE` | `production`（Cloud Logging 互換 JSON）/ `local`（TextHandler）|

ローカルで Pub/Sub / Firestore emulator に接続する場合は `PUBSUB_EMULATOR_HOST` / `FIRESTORE_EMULATOR_HOST` を併せて設定する（`make run` が既定値を渡す）。

## 公開パッケージ

[packages/api-account/](packages/api-account/) に REST 契約型（`apiaccount.PlayerResponse` 等）を公開している。
[data/openapi.yaml](data/openapi.yaml) が SSoT。編集後に以下で `openapi_gen.go` を再生成する。

```bash
scripts/generate_types.sh
```

クライアント向け TypeScript 型は `@kenyamaneko/overload-party-api-gateway` に統合済み。
