#!/usr/bin/env bash
# ci-status.sh — Concise CI check inspector for Claude and humans.
#
# Usage:
#   scripts/ci-status.sh              # Latest run for current branch
#   scripts/ci-status.sh --pr 127     # Checks for a specific PR
#   scripts/ci-status.sh --failed-only # Only show failed jobs with log tail
#   scripts/ci-status.sh --run 12345  # Specific run ID
set -euo pipefail

PR=""
RUN_ID=""
FAILED_ONLY=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pr) PR="$2"; shift 2 ;;
    --run) RUN_ID="$2"; shift 2 ;;
    --failed-only) FAILED_ONLY=true; shift ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# Determine which run to inspect
if [[ -n "$RUN_ID" ]]; then
  true # use provided run ID
elif [[ -n "$PR" ]]; then
  RUN_ID=$(gh run list --json databaseId,headBranch --limit 1 \
    --jq ".[0].databaseId" \
    -b "$(gh pr view "$PR" --json headRefName --jq '.headRefName')" 2>/dev/null || true)
else
  BRANCH=$(git branch --show-current)
  RUN_ID=$(gh run list --branch "$BRANCH" --limit 1 --json databaseId --jq '.[0].databaseId' 2>/dev/null || true)
fi

if [[ -z "$RUN_ID" ]]; then
  echo "No CI runs found."
  exit 0
fi

# Show job summary
echo "=== CI Run $RUN_ID ==="
gh run view "$RUN_ID" --json jobs --jq '.jobs[] | "\(if .conclusion == "success" then "✓" elif .conclusion == "failure" then "✗" elif .status == "in_progress" then "⋯" else "○" end) \(.name) (\(.conclusion // .status)) \(if .conclusion == "success" then "" else "" end)"'

# Show failed job logs
if [[ "$FAILED_ONLY" == true ]]; then
  FAILED=$(gh run view "$RUN_ID" --json jobs --jq '.jobs[] | select(.conclusion == "failure") | .name')
  if [[ -n "$FAILED" ]]; then
    echo ""
    echo "=== Failed Job Logs ==="
    gh run view "$RUN_ID" --log-failed 2>/dev/null | tail -50
  fi
fi
