#!/usr/bin/env bash
# Bootstrap a fresh Bitbucket Data Center instance for CI.
#
# The Bitbucket container auto-applies the license from BITBUCKET_LICENSE_KEY,
# so the only remaining setup step is creating the initial admin user.
# This script:
#   1. Polls /status until FIRST_RUN (license applied, user step pending)
#      or RUNNING (already bootstrapped).
#   2. On FIRST_RUN: fetches the setup page to obtain the session cookie and
#      CSRF token (atl_token), then POSTs the admin-user form.
#   3. Polls /status until RUNNING.
#
# Usage:
#   bash scripts/bootstrap-bitbucket.sh [base_url] [username] [password]
#
# Arguments (all optional, with defaults):
#   base_url  - Bitbucket base URL (default: http://localhost:7990)
#   username  - Admin username to create  (default: admin)
#   password  - Admin password to create  (default: admin)

set -euo pipefail

BASE_URL="${1:-http://localhost:7990}"
ADMIN_USERNAME="${2:-admin}"
ADMIN_PASSWORD="${3:-admin}"
ADMIN_EMAIL="admin@example.com"
ADMIN_DISPLAY_NAME="Admin"
COOKIE_JAR="$(mktemp /tmp/bb_bootstrap_cookies.XXXXXX)"
trap 'rm -f "$COOKIE_JAR"' EXIT

MAX_WAIT_SECONDS=300
POLL_INTERVAL=5

log() { echo "[bootstrap-bitbucket] $*" >&2; }

# Print the Bitbucket server state (STARTING, FIRST_RUN, RUNNING, or empty on
# error) by querying /status.
query_state() {
  local response
  response=$(curl -sf "${BASE_URL}/status" 2>/dev/null) || { echo ""; return 0; }
  python3 -c "
import sys, json
try:
    print(json.loads(sys.argv[1]).get('state', ''))
except Exception:
    print('')
" "$response"
}

# Poll /status until one of the expected states is reached.
# Prints the matched state to stdout.  Exits non-zero on timeout.
wait_for_states() {
  local elapsed=0
  while true; do
    local state
    state=$(query_state)
    log "State: ${state:-UNAVAILABLE} (${elapsed}s elapsed)"
    for target in "$@"; do
      if [ "$state" = "$target" ]; then
        echo "$state"
        return 0
      fi
    done
    if [ "$elapsed" -ge "$MAX_WAIT_SECONDS" ]; then
      log "Timeout after ${MAX_WAIT_SECONDS}s waiting for state(s): $*"
      return 1
    fi
    sleep "$POLL_INTERVAL"
    elapsed=$((elapsed + POLL_INTERVAL))
  done
}

log "Waiting for Bitbucket to be ready (${BASE_URL})..."
current_state=$(wait_for_states "FIRST_RUN" "RUNNING")

if [ "$current_state" = "FIRST_RUN" ]; then
  log "Fetching setup page to obtain session cookie and CSRF token..."
  setup_html=$(curl -sf -c "$COOKIE_JAR" "${BASE_URL}/setup" 2>/dev/null)

  atl_token=$(python3 -c "
import sys, re
html = sys.argv[1]
# atl_token appears as either name=...value=... or value=...name=...
for pat in [r\"name=['\\\"]atl_token['\\\"][^>]*value=['\\\"]([^'\\\"]*)\",
            r\"value=['\\\"]([^'\\\"]*)['\\\"][^>]*name=['\\\"]atl_token['\\\"]\"]:
    m = re.search(pat, html)
    if m:
        print(m.group(1))
        sys.exit(0)
sys.exit(1)
" "$setup_html")
  log "Got CSRF token."

  log "Creating admin user '${ADMIN_USERNAME}'..."
  http_code=$(
    curl -sf -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
      -o /dev/null -w "%{http_code}" \
      -X POST "${BASE_URL}/setup" \
      --data-urlencode "step=user" \
      --data-urlencode "username=${ADMIN_USERNAME}" \
      --data-urlencode "fullname=${ADMIN_DISPLAY_NAME}" \
      --data-urlencode "email=${ADMIN_EMAIL}" \
      --data-urlencode "password=${ADMIN_PASSWORD}" \
      --data-urlencode "confirmPassword=${ADMIN_PASSWORD}" \
      --data-urlencode "skipJira=" \
      --data-urlencode "atl_token=${atl_token}"
  )
  log "Setup POST HTTP status: ${http_code}"

  log "Waiting for Bitbucket to reach RUNNING state..."
  wait_for_states "RUNNING"
fi

log "Bitbucket is RUNNING at ${BASE_URL} (admin user: ${ADMIN_USERNAME})"
