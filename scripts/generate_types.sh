#!/usr/bin/env bash
# generate_types.sh — data/openapi.yaml から packages/api-account の Go 型を再生成する。
#
# account は subscriber のみで自身は Pub/Sub event を publish しないため、AsyncAPI 生成は無い。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$REPO_ROOT/packages/api-account"
oapi-codegen -config openapi-codegen.yaml ../../data/openapi.yaml
