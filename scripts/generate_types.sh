#!/usr/bin/env bash
# generate_types.sh — data/openapi.yaml から packages/api-account (Go) と packages/api-account-npm (TS) の型を再生成する。
#
# account は subscriber のみで自身は Pub/Sub event を publish しないため、AsyncAPI 生成は無い。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$REPO_ROOT/packages/api-account"
oapi-codegen -config openapi-codegen.yaml ../../data/openapi.yaml

cd "$REPO_ROOT"
npx --yes openapi-typescript@7 \
  data/openapi.yaml \
  --output packages/api-account-npm/src/openapi.gen.ts
