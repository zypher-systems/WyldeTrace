#!/bin/sh
# Link: SPA shell and documented health/login entry points respond.
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"

check_html() {
  path="$1"
  code="$(curl -sS -o /tmp/wt-link-body -w '%{http_code}' --max-time 15 "${BASE_URL}${path}")"
  if [ "$code" != "200" ]; then
    echo "GET ${path} -> ${code}" >&2
    exit 1
  fi
  grep -qi 'WyldeTrace' /tmp/wt-link-body || {
    echo "GET ${path} missing WyldeTrace title/brand" >&2
    exit 1
  }
  echo "link ok: GET ${path} (${code})"
}

check_html "/"
check_html "/login"
check_html "/subnets"
check_html "/inbox"
