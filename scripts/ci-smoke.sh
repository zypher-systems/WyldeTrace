#!/bin/sh
# Smoke: process is up and the database is reachable.
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"

body="$(curl -fsS --max-time 15 "${BASE_URL}/healthz")"
echo "$body" | grep -q '"status":"ok"' || {
  echo "healthz missing status=ok: $body" >&2
  exit 1
}
echo "$body" | grep -q '"db":true' || {
  echo "healthz db is not true: $body" >&2
  exit 1
}
echo "smoke ok: ${BASE_URL}/healthz"
