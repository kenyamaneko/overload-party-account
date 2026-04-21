# Overload Party - Development Principles

## 設計思想
- エラーは握りつぶさず根本解決する。ログだけして握りつぶすのは禁止
- デフォルト値へのフォールバックを行わない。意図しない値になるならエラーにする
- 一つの関数に複数の責務を負わせない
- 保守性と拡張性を最大限に高めるため、クリーンアーキテクチャを遵守する
  - ビジネスロジックは外部アダプターに依存しない
  - 純粋な port や delivery 層とロジックを分離する
  - リポジトリ層は純粋な外部永続装置へのアクセスであり、ロジックを持たない

## コーディング方針
- 関数などには docs コメントを付与する
- 実装との二重管理になるので What コメントを書いてはいけない
- 実装から読み取れない意図は Why コメントとして記述する。設計レベルの意図なら
  ARCHITECTURE.md に記述する

## ログ方針
- 構造化されたログを出力する
- ログ出力すべき事象
  - 呼び出し元に伝搬させられないエラーの補足
  - エラーではないが、その場所でしか気づけない事象 (未対応のイベント種別など)
- ログレベル
  - `Info`: 起動・停止・正常系の事象
  - `Warn`: システムの運用に影響を与えない、あるいは影響が軽微な事象
  - `Error`: システムの運用に支障をきたす事象

## テスト方針
- 実装をなぞるテストではなく、仕様に従っていることを確認するテストを書く
- 「この関数がこう動く」ではなく「この仕様をこう満たす」という観点で書く
- テストコードはテーブル駆動で書く
- ケースの網羅性が見えづらくなるため、テストコードに if 文を書かず、条件はケースで表現する

## API契約
- エンドポイント・イベント名・ヘッダ名・トピック名・サブスクリプション名・ファクション名はリテラルで書かず
  共通パッケージの定数を使う

## ブランチ・Issue 運用
- 開発は GitHub Issue を起点に行う
- Issue / コミットメッセージの type は以下のいずれかを使う:
  `feat` / `fix` / `refactor` / `docs` / `chore` / `test` / `perf` / `ci`
  - Issue タイトル: `[{type}] {要約}` (日本語・50 文字以内)
  - コミットメッセージ: `{type}: {要約}` (Conventional Commits)
- feature ブランチは develop から切る(命名: `feature/{issue番号}-{概要}`)
- hotfix は main から切る(develop ではない)
- ブランチ戦略の詳細は [BRANCHING.md](docs/BRANCHING.md) を参照
- 詳細手順は Skills を参照

## 禁止事項
- git tag の手動打ちは禁止（CI が自動生成）
- 生成済み型コード（`packages/api-account/*_gen.go`）を手で書き換えない。型は
  `data/models.yaml` を SSoT とし、変更後は `python3 scripts/generate_types.py` で再生成する
- このファイル（CLAUDE.md）を Claude が書き換えない。
  ルールの追加・修正は人間が明示的に指示した場合のみ行う
- クライアント認証をこのサービスで行わない。account は ClusterIP のみで公開され、
  gateway が Firebase 認証済みの `playerId` / `firebase_uid` を forward する前提で動作する
- `DATABASE_URL` 未設定時のフォールバックを再導入しない。未設定なら起動を fail させる
  （`PUBSUB_PROJECT_ID` / `FIRESTORE_PROJECT_ID` も同様）
- `auth_service.Register` にスターターカード付与や初期ファクション付与を再導入しない。
  初期ファクション選択・カード配布のトリガーは scenario 側の責務に移動済み
- `factions` リファレンステーブルを再導入しない。ファクションマスターの SSoT は
  `common/data/factions.yaml` から code-generate された定数（`gamedesign.SelectableFactions` 等）
