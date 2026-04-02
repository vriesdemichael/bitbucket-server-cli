#!/usr/bin/env bash
# Bootstrap a fresh Bitbucket Data Center instance for CI.
#
# Polls /status until the server is ready (FIRST_RUN or RUNNING), then
# completes the setup wizard by creating the initial admin user via the
# /setup form endpoint.  The license and database are already wired via
# container environment variables (BITBUCKET_LICENSE_KEY + JDBC_*), so
# only the admin-user step needs to be driven here.
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

MAX_WAIT_SECONDS=300
POLL_INTERVAL=5

log() { echo "[bootstrap-bitbucket] $*" >&2; }

# Print the Bitbucket server state (STARTING, FIRST_RUN, RUNNING, or empty on
# error) by querying /status.  All output goes to stdout so callers can
# capture it; informational messages go to stderr via log().
query_state() {
  local response
  response=$(curl -sf "${BASE_URL}/status" 2>/dev/null) || { echo ""; return 0; }
  python3 - <<'EOF' "$response"
import sys, json
try:
    print(json.loads(sys.argv[1]).get("state", ""))
except Exception:
    print("")
EOF
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
  log "Completing setup wizard: creating admin user '${ADMIN_USERNAME}'..."

  http_code=$(
    curl -s -o /dev/null -w "%{http_code}" \
      -X POST "${BASE_URL}/setup" \
      -H "Content-Type: application/x-www-form-urlencoded" \
      --data-urlencode "step=user-install" \
      --data-urlencode "user.username=${ADMIN_USERNAME}" \
      --data-urlencode "user.password=${ADMIN_PASSWORD}" \
      --data-urlencode "user.confirmPassword=${ADMIN_PASSWORD}" \
      --data-urlencode "user.displayName=${ADMIN_DISPLAY_NAME}" \
      --data-urlencode "user.emailAddress=${ADMIN_EMAIL}" \
      --data-urlencode "baseUrl=${BASE_URL}"
  )
  log "Setup POST HTTP status: ${http_code}"

  log "Waiting for Bitbucket to reach RUNNING state..."
  wait_for_states "RUNNING"
fi

log "Bitbucket is RUNNING at ${BASE_URL} (admin user: ${ADMIN_USERNAME})"
