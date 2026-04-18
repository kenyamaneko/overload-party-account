# Branching Strategy

本リポジトリのブランチ戦略とリリース運用を定義する。

> **Note**: このドキュメントは将来 `overload-party-common` に移動する予定。他リポジトリの main ブランチの品質が安定した段階で、共通ルールとして参照される形にする。

## 概要

GitFlow をベースに、環境とブランチを対応付けた運用を採用する。account はプレイヤー情報とファクション所有を所有する中核サービスで、DB スキーマの破壊的変更や Pub/Sub イベント契約の変更が他サービス（shop / scenario / battle / gateway）に伝播するため、stg 環境での cross-service 検証を挟む昇格モデルが必須となる。

## ブランチ一覧

| ブランチ | 環境 | 寿命 | 派生元 | マージ先 | 保護 |
|---|---|---|---|---|---|
| `main` | prod | 永続 | — | — | 最大 |
| `release/vX.Y.Z` | stg | 短命 | `develop` | `main` | あり |
| `develop` | dev | 永続 | `main` (初回のみ) | — | あり |
| `feature/xxx` | なし | 短命 | `develop` | `develop` | なし |
| `hotfix/xxx` | なし | 短命 | `main` | `main` + `develop` (+ `release` if exists) | なし |

## ブランチ運用ルール

### main

- **prod 環境のソース・オブ・トゥルース**。main の HEAD = prod で動作しているコード
- 直 push 禁止。PR 経由のマージのみ
- マージ元として許可するのは `release/*` と `hotfix/*` のみ
- `develop` や `feature/*` を直接 main にマージしない
- タグは CI が自動で打つ（手動タグ付け禁止）
- force push 禁止、履歴書き換え禁止

### develop

- **dev 環境のソース**。次リリースに向けた統合ブランチ
- 直 push 禁止。PR 経由のマージのみ
- マージ元として許可するのは `feature/*` と `hotfix/*` の back-merge
- CI green 必須。レビューは self-approve 可（速度優先）

### release/vX.Y.Z

- **stg 環境のソース**。リリース候補の検証ブランチ
- 短命。main にマージ後、削除する
- ブランチ名に候補バージョンを含める（例: `release/v1.2.0`）
- `develop` から切る。切った時点で feature の取り込みは停止する
- release 中に feature を追加で取り込みたい場合は、原則として次の release に回す
- バグ修正やリリース準備（CHANGELOG 更新等）のコミットは PR 経由で release に入れる
- release に入れた修正は、main マージ後に develop にも back-merge する（後述）

### feature/xxx

- 新機能・改善の作業ブランチ
- `develop` から切って `develop` にマージ
- 命名例: `feature/add-username-validation`, `feature/account/issue-42`
- PR マージ時にブランチ削除

### hotfix/xxx

- **prod 緊急修正**の作業ブランチ
- `main` から切る（develop からではない — develop には未リリース変更が混ざっているため）
- main と develop の両方にマージする（back-merge 必須）
- release ブランチが存在する場合は、release にもマージする
- 命名例: `hotfix/fix-register-duplicate`, `hotfix/pubsub-leak`

## リリースフロー

### 通常リリース

```
1. develop で feature を統合・dev 環境で検証
   └─ feature/xxx → develop (PR)

2. release ブランチを切る
   └─ git switch -c release/v1.2.0 develop
   └─ push → stg 環境に自動デプロイ

3. stg 環境で検証
   └─ 他サービスとの Pub/Sub 疎通、Register/Login フロー、デイリーバトル reset 等
   └─ バグ発見時は PR 経由で release ブランチに修正を入れる

4. main にマージ
   └─ release/v1.2.0 → main (PR)
   └─ CI が自動でタグ v1.2.0 を打つ
   └─ main が prod 環境に自動デプロイ

5. develop に back-merge
   └─ release/v1.2.0 → develop (PR)
   └─ release 中に入れた修正を develop に戻す

6. release ブランチ削除
```

### hotfix リリース

```
1. hotfix ブランチを切る
   └─ git switch -c hotfix/fix-battle-limit-reset main

2. 修正 → PR → main にマージ
   └─ hotfix/xxx → main (PR)
   └─ CI が自動でタグ v1.2.1 を打つ（patch bump）
   └─ prod 環境に自動デプロイ

3. develop に back-merge（必須）
   └─ hotfix/xxx → develop (PR)

4. release ブランチが存在する場合は release にも back-merge
   └─ hotfix/xxx → release/vX.Y.Z (PR)

5. hotfix ブランチ削除
```

### hotfix の back-merge 忘れ対策

hotfix を main にマージしたが develop に戻し忘れると、次のリリースでバグが再発する。

対策:

- PR テンプレートに back-merge チェックリストを入れる
- main に hotfix が入ったら、CI で develop への back-merge PR を自動生成する workflow を用意する（未作成）

## バージョニング

Semantic Versioning (SemVer) を採用する。

- **MAJOR**: 破壊的変更（REST API スキーマ破壊、DB マイグレーション、Pub/Sub イベントスキーマ破壊等、既存呼び出し元が動かなくなる変更）
- **MINOR**: 後方互換のある機能追加
- **PATCH**: バグ修正、ドキュメント修正、内部リファクタ

### サービス本体のタグ

サービス本体のタグは [.github/workflows/release-tag.yaml](../.github/workflows/release-tag.yaml) が main への PR マージ時に自動で打つ。

- release マージ時: ブランチ名からバージョンを取得（`release/v1.2.0` → `v1.2.0`）
- hotfix マージ時: 最新タグから patch を自動 bump（`v1.2.0` → `v1.2.1`）

**手動タグ禁止**。CLAUDE.md 禁止事項にも記載。

### `packages/api-account` のタグ

`packages/api-account` は Go module として独立のバージョンを持つ。タグ形式は:

```
packages/api-account/vX.Y.Z
```

発行は [.github/workflows/publish.yaml](../.github/workflows/publish.yaml) が担当し、`workflow_dispatch` でのみ実行する（人が bump 種別を判断する運用）。`bump=patch|minor|major` を明示指定する。

`api-account` のバージョンはサービス本体のバージョンと独立で、REST 契約型に破壊的変更が入る release では major bump を推奨する。

## ブランチ保護設定

GitHub Rulesets で以下を設定する（shop と同等）。

### main

- 直 push 禁止
- PR マージのみ許可（linear history）
- force push 禁止、削除禁止
- 履歴書き換え禁止
- 必須ステータスチェック: CI / lint, CI / test が green
- required reviews: 1（self-approve 不可）
- マージ元ブランチ制限: `release/*` と `hotfix/*` のみ

### release/*

- 直 push 禁止。PR 経由のマージのみ
- force push 禁止、削除は手動で可
- 必須ステータスチェック: CI / lint, CI / test が green

### develop

- 直 push 禁止
- PR マージのみ許可
- 必須ステータスチェック: CI / lint, CI / test が green
- required reviews: 不要（一人開発での速度優先）

## CI/CD パイプライン

| ワークフロー | トリガー | 役割 |
|---|---|---|
| `ci.yaml` | PR: main / develop / release/** | check-source-branch / lint / test / image-scan / codegen-sync。ブランチ保護の required status check として使う |
| `deploy.yaml` | push: main / develop / release/** | ブランチ → 環境解決（main=prod, release/*=stg, develop=dev）→ build & push to Artifact Registry |
| `release-tag.yaml` | PR closed (merged) to main | `release/*` または `hotfix/*` が main にマージされた時に SemVer タグを自動作成 |
| `publish.yaml` | workflow_dispatch | `packages/api-account` Go モジュールの SemVer タグ付け・公開（bump=patch/minor/major を入力で指定） |

CI と CD は完全に分離している。CI は PR ゲート（品質保証）のみを行い、実際の push は `deploy.yaml` が担当する。CI の成功はブランチ保護の required status check で担保し、マージされたらすぐ `deploy.yaml` が対応環境にデプロイする。

### feature / hotfix ブランチの CI

feature/* や hotfix/* ブランチへの直 push では CI が走らない。CI を実行するには、対象ブランチ（develop / main / release/**）宛の PR を作成する。PR 更新時（追加 push）にも CI が再実行される。

### main へのマージ源の制約

`ci.yaml` の `check-source-branch` ジョブが、main 向け PR の head ブランチを `release/*` または `hotfix/*` に限定する。`develop` や `feature/*` を直接 main にマージする PR はこのジョブで fail する。
