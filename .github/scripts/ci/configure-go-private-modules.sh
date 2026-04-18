#!/usr/bin/env bash
# configure-go-private-modules.sh — プライベート Go モジュールへのアクセスを設定する。
# account は overload-party-common と overload-party-battle の 2 つを参照するため
# 両方の insteadOf を設定する。
# 入力: COMMON_GO_MODULES_FETCH, BATTLE_GO_MODULES_FETCH (env)
set -euo pipefail

if [ -z "${COMMON_GO_MODULES_FETCH:-}" ]; then
  echo "::error::COMMON_GO_MODULES_FETCH is not set"
  exit 1
fi
if [ -z "${BATTLE_GO_MODULES_FETCH:-}" ]; then
  echo "::error::BATTLE_GO_MODULES_FETCH is not set"
  exit 1
fi

git config --global url."https://x-access-token:${COMMON_GO_MODULES_FETCH}@github.com/kenyamaneko/overload-party-common".insteadOf "https://github.com/kenyamaneko/overload-party-common"
git config --global url."https://x-access-token:${BATTLE_GO_MODULES_FETCH}@github.com/kenyamaneko/overload-party-battle".insteadOf "https://github.com/kenyamaneko/overload-party-battle"
