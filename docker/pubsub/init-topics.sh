#!/bin/bash
# account が購読する Pub/Sub topic + subscription を emulator に作成する。
# 実環境では各 publisher サービスが topic を作成するが、account 単体起動では
# publisher が居ないため、購読対象の topic をここで用意して subscription を張る。
set -euo pipefail

PROJECT="${PUBSUB_PROJECT_ID:?compose 経由で PUBSUB_PROJECT_ID を設定すること}"
HOST="${PUBSUB_EMULATOR_HOST:?compose 経由で PUBSUB_EMULATOR_HOST を設定すること}"

# pull subscription の ack 期限 (秒)。account は起動時に stream を開くだけで消費挙動には効かないため、ローカルでは実環境の既定に合わせる。
readonly ACK_DEADLINE_SECS=30

# 既存 (409) は idempotent 再実行として許容し、それ以外の HTTP 失敗は abort する。
put() {
  local url="$1"
  shift
  local code
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X PUT "$url" "$@")
  case "$code" in
    2*|409) return 0 ;;
    *) echo "PUT $url -> HTTP $code" >&2; return 1 ;;
  esac
}

# 各行は「topic account-subscription」。実環境で topic を publish するのは別サービスだが、
# account 単体起動では購読先を成立させるために topic ごと用意する。
while read -r topic sub; do
  put "http://${HOST}/v1/projects/${PROJECT}/topics/${topic}"
  put "http://${HOST}/v1/projects/${PROJECT}/subscriptions/${sub}" \
    -H "Content-Type: application/json" \
    -d "{\"topic\":\"projects/${PROJECT}/topics/${topic}\",\"ackDeadlineSeconds\":${ACK_DEADLINE_SECS}}"
  echo "ready: ${sub} -> ${topic}"
done <<EOF
faction-acquired       faction-acquired-account-sub
premium-updated        premium-updated-account-sub
player-onboarded       player-onboarded-account-sub
onboarding-name-set    onboarding-name-set-account-sub
onboarding-faction-set onboarding-faction-set-account-sub
EOF
