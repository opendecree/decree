#!/usr/bin/env bash
# check-coverage.sh — Enforce per-module coverage thresholds (ratchet).
#
# Usage:
#   ./scripts/check-coverage.sh          # Check coverage against thresholds
#   ./scripts/check-coverage.sh --update  # Update thresholds to current values (ratchet up only)
set -euo pipefail

THRESHOLDS_FILE="coverage-thresholds.json"

# Module → test pattern pairs.
declare -A MODULES=(
  ["internal"]="./internal/..."
  ["sdk/configclient"]="./..."
  ["sdk/adminclient"]="./..."
  ["sdk/configwatcher"]="./..."
  ["sdk/tools"]="./..."
  ["cmd/decree"]="./..."
)

# Module → working directory (empty = repo root).
declare -A MODULE_DIRS=(
  ["internal"]="."
  ["sdk/configclient"]="sdk/configclient"
  ["sdk/adminclient"]="sdk/adminclient"
  ["sdk/configwatcher"]="sdk/configwatcher"
  ["sdk/tools"]="sdk/tools"
  ["cmd/decree"]="cmd/decree"
)

get_coverage() {
  local dir=$1
  local pattern=$2
  local cov

  if [ "$dir" = "." ]; then
    go test "$pattern" -coverprofile=coverage.tmp -count=1 > /dev/null 2>&1 || true
    cov=$(go tool cover -func=coverage.tmp 2>/dev/null | awk '/^total:/ {gsub(/%/,""); print $NF}')
    rm -f coverage.tmp
  else
    pushd "$dir" > /dev/null
    go test "$pattern" -coverprofile=coverage.tmp -count=1 > /dev/null 2>&1 || true
    cov=$(go tool cover -func=coverage.tmp 2>/dev/null | awk '/^total:/ {gsub(/%/,""); print $NF}')
    rm -f coverage.tmp
    popd > /dev/null
  fi

  echo "${cov:-0}"
}

get_threshold() {
  local module=$1
  if [ ! -f "$THRESHOLDS_FILE" ]; then
    echo "0"
    return
  fi
  local val
  val=$(python3 -c "import json; d=json.load(open('$THRESHOLDS_FILE')); print(d.get('$module', 0))" 2>/dev/null)
  echo "${val:-0}"
}

# Collect all coverage values.
declare -A COVERAGES
for module in "${!MODULES[@]}"; do
  COVERAGES[$module]=$(get_coverage "${MODULE_DIRS[$module]}" "${MODULES[$module]}")
done

if [ "${1:-}" = "--update" ]; then
  # Ratchet: update thresholds, only if coverage improved.
  echo "{"
  first=true
  for module in $(echo "${!MODULES[@]}" | tr ' ' '\n' | sort); do
    current=${COVERAGES[$module]}
    old=$(get_threshold "$module")
    # Use the higher of current and old (ratchet up only).
    new=$(python3 -c "print(max(float('$current'), float('$old')))")
    # Floor to one decimal.
    new=$(python3 -c "import math; print(math.floor(float('$new') * 10) / 10)")
    if [ "$first" = true ]; then first=false; else echo ","; fi
    printf '  "%s": %s' "$module" "$new"
  done
  echo ""
  echo "}"
  exit 0
fi

# Check mode: compare against thresholds.
failed=0
for module in $(echo "${!MODULES[@]}" | tr ' ' '\n' | sort); do
  current=${COVERAGES[$module]}
  threshold=$(get_threshold "$module")
  status="✓"
  if python3 -c "exit(0 if float('$current') >= float('$threshold') else 1)" 2>/dev/null; then
    status="✓"
  else
    status="✗ BELOW THRESHOLD"
    failed=1
  fi
  printf "%-25s %6s%% (threshold: %s%%) %s\n" "$module" "$current" "$threshold" "$status"
done

if [ "$failed" -eq 1 ]; then
  echo ""
  echo "Coverage regression detected. Fix the failing modules or update thresholds with:"
  echo "  ./scripts/check-coverage.sh --update > coverage-thresholds.json"
  exit 1
fi

echo ""
echo "All modules meet coverage thresholds."
