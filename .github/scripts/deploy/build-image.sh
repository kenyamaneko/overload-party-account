#!/usr/bin/env bash
# build-image.sh — Docker イメージをビルドする。
# account は overload-party-common と overload-party-battle の両方を private module として参照するため
# BuildKit secret を 2 つ渡す。
# 入力: IMAGE, SHA_TAG, LATEST_TAG, COMMON_GO_MODULES_FETCH, BATTLE_GO_MODULES_FETCH (env)
set -euo pipefail

docker build \
  --secret id=COMMON_GO_MODULES_FETCH,env=COMMON_GO_MODULES_FETCH \
  --secret id=BATTLE_GO_MODULES_FETCH,env=BATTLE_GO_MODULES_FETCH \
  -t "${IMAGE}:${SHA_TAG}" \
  -t "${IMAGE}:${LATEST_TAG}" \
  .
