#!/usr/bin/env bash
#
# Compose, preflight, and publish one post from a file — the loop a scheduled
# job would run. Every step uses --json so the script reads structured output
# rather than parsing a table.
#
#   export FOPOST_API_KEY=fp_...
#   ./examples/daily-digest.sh digest.md
#
# Requires: fopost, jq.

set -euo pipefail

TEXT_FILE="${1:?usage: daily-digest.sh <text-file> [media-file]}"
MEDIA_FILE="${2:-}"

command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

# Exit codes the CLI uses, so this script can react to each one by name.
readonly EXIT_PAYMENT_REQUIRED=4
readonly EXIT_RATE_LIMITED=7

# --- 1. Pick the workspace -------------------------------------------------
# `auth login --workspace` can save a default; this works without one.
WORKSPACE=$(fopost workspaces list --json | jq -r '.[0].id')
[ -n "$WORKSPACE" ] && [ "$WORKSPACE" != "null" ] || {
  echo "No workspace is reachable with this key." >&2
  exit 1
}
echo "Workspace: $WORKSPACE"

# --- 2. Refuse to post through a broken connection -------------------------
UNHEALTHY=$(fopost accounts health --workspace "$WORKSPACE" --json \
  | jq -r '.accounts[] | select(.healthStatus != "healthy") | "\(.platform)/\(.username)"')

if [ -n "$UNHEALTHY" ]; then
  echo "These accounts need reconnecting, and are skipped:" >&2
  echo "$UNHEALTHY" | sed 's/^/  - /' >&2
fi

# read -r in a loop rather than mapfile, so this runs on the bash 3.2 macOS ships.
ACCOUNT_FLAGS=()
ACCOUNT_COUNT=0
while IFS= read -r account; do
  [ -n "$account" ] || continue
  ACCOUNT_FLAGS+=(--account "$account")
  ACCOUNT_COUNT=$((ACCOUNT_COUNT + 1))
done < <(
  fopost accounts health --workspace "$WORKSPACE" --json \
    | jq -r '.accounts[] | select(.healthStatus == "healthy") | .id'
)
[ "$ACCOUNT_COUNT" -gt 0 ] || { echo "No healthy account to post to." >&2; exit 1; }
echo "Posting to $ACCOUNT_COUNT account(s)."

# --- 3. Create the draft ---------------------------------------------------
CREATE_FLAGS=(--workspace "$WORKSPACE" "${ACCOUNT_FLAGS[@]}" --text-file "$TEXT_FILE" --draft --json)
[ -n "$MEDIA_FILE" ] && CREATE_FLAGS+=(--media "$MEDIA_FILE")

set +e
CREATED=$(fopost posts create "${CREATE_FLAGS[@]}")
STATUS=$?
set -e

case "$STATUS" in
  0) ;;
  "$EXIT_PAYMENT_REQUIRED") echo "The plan does not cover this. Upgrade and re-run." >&2; exit "$STATUS" ;;
  "$EXIT_RATE_LIMITED")     echo "Rate limited. Re-run after the window resets." >&2;     exit "$STATUS" ;;
  *)                        echo "Could not create the post (exit $STATUS)." >&2;         exit "$STATUS" ;;
esac

POST_ID=$(echo "$CREATED" | jq -r '.post.id')
echo "Draft: $POST_ID"

# --- 4. Preflight before anything leaves ------------------------------------
# preflight exits non-zero when a platform would reject the post.
if ! PREFLIGHT=$(fopost posts preflight "$POST_ID" --json); then
  echo "Preflight found blockers; the draft was left in place:" >&2
  echo "$PREFLIGHT" \
    | jq -r '.accounts[] | select(.ready == false) | "  \(.platform): \(.issues | join("; "))"' >&2
  exit 1
fi

echo "$PREFLIGHT" \
  | jq -r '.accounts[].signals[]? | "  note · \(.message)"'

# --- 5. Publish, then report per account ------------------------------------
fopost posts publish "$POST_ID" --json \
  | jq -r '.deliveries[] | "  \(.accountId): \(.status)"'

echo "Queued. Deliveries land asynchronously — poll for the outcome:"
echo "  fopost posts deliveries $POST_ID"
