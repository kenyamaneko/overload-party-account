# CLAUDE.md - overload-party-account

## 行動制約

- エラーは握りつぶさない
- git tag を手動で打たない（CI が自動作成する）
- TODO スタブを追加しない
- クライアント認証を行わない（ClusterIP のみ、gateway が認証済みの playerId を forward する）
- `DATABASE_URL` 未設定時のフォールバックを再導入しない（起動を fail させる）
- `packages/api-account/*_gen.go` を直接編集しない（`data/models.yaml` → codegen）
- `auth_service.Register` にスターター付与呼び出しを再導入しない（トリガーは scenario に移動済み）
- `factions` リファレンステーブルを再導入しない（`data/factions.yaml` → code-generated 定数が SSoT）
- 型変更時は `data/models.yaml` → `python3 scripts/generate_types.py` を実行する
