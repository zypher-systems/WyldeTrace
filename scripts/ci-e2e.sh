#!/bin/sh
# API e2e against a running instance: login, inventory list, create subnet + IP.
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
EMAIL="${ADMIN_EMAIL:-admin@wyldetech.local}"
PASSWORD="${ADMIN_PASSWORD:-changeme}"

req() {
  method="$1"
  path="$2"
  token="${3:-}"
  data="${4:-}"
  if [ -n "$data" ] && [ -n "$token" ]; then
    curl -sS --max-time 20 -X "$method" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${token}" \
      -d "$data" \
      -o /tmp/wt-e2e-body \
      -w '%{http_code}' \
      "${BASE_URL}${path}"
  elif [ -n "$data" ]; then
    curl -sS --max-time 20 -X "$method" \
      -H "Content-Type: application/json" \
      -d "$data" \
      -o /tmp/wt-e2e-body \
      -w '%{http_code}' \
      "${BASE_URL}${path}"
  elif [ -n "$token" ]; then
    curl -sS --max-time 20 -X "$method" \
      -H "Authorization: Bearer ${token}" \
      -o /tmp/wt-e2e-body \
      -w '%{http_code}' \
      "${BASE_URL}${path}"
  else
    curl -sS --max-time 20 -X "$method" \
      -o /tmp/wt-e2e-body \
      -w '%{http_code}' \
      "${BASE_URL}${path}"
  fi
}

expect() {
  got="$1"
  want="$2"
  label="$3"
  if [ "$got" != "$want" ]; then
    echo "${label}: status ${got} (want ${want})" >&2
    cat /tmp/wt-e2e-body >&2
    exit 1
  fi
  echo "e2e ok: ${label} (${got})"
}

json_string() {
  key="$1"
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p" /tmp/wt-e2e-body | head -n 1
}

code="$(req POST /api/v1/auth/login "" "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"
expect "$code" "200" "login"
token="$(json_string token)"
if [ -z "$token" ]; then
  echo "login response missing token" >&2
  cat /tmp/wt-e2e-body >&2
  exit 1
fi

code="$(req GET /api/v1/auth/me "$token")"
expect "$code" "200" "auth/me"

code="$(req GET /api/v1/locations "$token")"
expect "$code" "200" "list locations"

code="$(req GET "/api/v1/subnets?page=1&page_size=10" "$token")"
expect "$code" "200" "list subnets"

code="$(req GET "/api/v1/inbox?page=1&page_size=10" "$token")"
expect "$code" "200" "list inbox"

cidr="198.51.100.0/24"
code="$(req POST /api/v1/subnets "$token" "{\"cidr\":\"${cidr}\",\"name\":\"CI E2E\"}")"
if [ "$code" != "201" ]; then
  cidr="198.51.$((100 + ${CI_JOB_ID:-1} % 50)).0/24"
  code="$(req POST /api/v1/subnets "$token" "{\"cidr\":\"${cidr}\",\"name\":\"CI E2E\"}")"
fi
expect "$code" "201" "create subnet"
subnet_id="$(json_string id)"
if [ -z "$subnet_id" ]; then
  echo "create subnet missing id" >&2
  cat /tmp/wt-e2e-body >&2
  exit 1
fi

host="${cidr%.0/24}.10"
code="$(req POST "/api/v1/subnets/${subnet_id}/ips" "$token" "{\"address\":\"${host}\",\"hostname\":\"ci-e2e\",\"status\":\"assigned\"}")"
expect "$code" "201" "create ip"

echo "e2e ok: login → inventory → ${cidr} + ${host}"
