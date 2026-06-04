#!/usr/bin/env bash
# check.sh — extract issue references from commits since main
# Outputs: export ISSUE_REFS="123 456" (space-separated, deduplicated)
# Exit 0 if refs found, exit 1 if none found

set -euo pipefail

log=$(git log --oneline main..HEAD 2>/dev/null || true)

if [[ -z "$log" ]]; then
  export ISSUE_REFS=""
  exit 1
fi

# Match patterns (case-insensitive):
#   #123
#   closes #123 / close #123
#   fixes #123 / fix #123
#   resolves #123 / resolve #123
#   refs #123 / ref #123
refs=$(echo "$log" \
  | grep -oiE '(closes?|fixes?|resolves?|refs?)[[:space:]]+#([0-9]+)|#([0-9]+)' \
  | grep -oE '[0-9]+' \
  | sort -nu \
  | tr '\n' ' ' \
  | sed 's/[[:space:]]*$//')

if [[ -z "$refs" ]]; then
  export ISSUE_REFS=""
  exit 1
fi

export ISSUE_REFS="$refs"
echo "export ISSUE_REFS=\"$refs\""
exit 0
