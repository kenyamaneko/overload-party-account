# overload-party-account

プレイヤー管理・設定・ファクション所有・経験値・対戦制限を担う内部マイクロサービス。

## サービス間連携

```
Gateway (主たる呼び出し元)
  ├─ POST /auth/register, /auth/login
  ├─ GET/PUT /players/:playerId/*
  └─ GET/PUT /players/:playerId/settings
                │
                ▼
Account (このサービス, :9005)
  ├─ PostgreSQL (account スキーマ所有)
  └─ Pub/Sub subscriber
       ├─ faction-selected  ← scenario / shop が publish
       └─ premium-updated   ← shop が publish

Battle (service-to-service)
  ├─ POST /players/:playerId/battle-limit/increment
  └─ POST /players/award-game-exp
```

- 認証はしない。Gateway が Firebase Auth 済みの playerId を forward する
- Pub/Sub subscriber が faction-selected / premium-updated イベントを受信し、account スキーマに反映する

エンドポイント一覧は [docs/API_REFERENCE.md](docs/API_REFERENCE.md) を参照。

## 環境変数

**Secret:**

| 変数名 | 説明 |
|---|---|
| `DATABASE_URL` | PostgreSQL 接続文字列 |

**ConfigMap:**

| 変数名 | デフォルト | 説明 |
|---|---|---|
| `PORT` | `9005` | リッスンポート |
| `ENV` | `dev` | `dev` / `stg` / `prod` |
| `PUBSUB_PROJECT_ID` | (必須) | Pub/Sub Google Cloud プロジェクト |
| `FACTION_SELECTED_SUBSCRIPTION` | `faction-selected-account-sub` | faction-selected Pub/Sub サブスクリプション名 |
| `PREMIUM_UPDATED_SUBSCRIPTION` | `premium-updated-account-sub` | premium-updated Pub/Sub サブスクリプション名 |

`DATABASE_URL` / `PUBSUB_PROJECT_ID` が未設定なら起動時に即 fail する。

## 公開パッケージ

| パッケージ | パス | 用途 |
|---|---|---|
| Go module | `packages/api-account/` | REST 契約型 (`apiaccount.PlayerResponse` 等) |

SSoT: `data/models.yaml` -> `python3 scripts/generate_types.py` で再生成。

クライアント向け TypeScript 型は `@kenyamaneko/overload-party-api-gateway` に統合済み。
