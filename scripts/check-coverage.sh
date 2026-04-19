#!/usr/bin/env bash
# check-coverage.sh — Enforce per-module coverage thresholds (ratchet).
#
# Usage:
#   ./scripts/check-coverage.sh           # Check coverage against thresholds
#   ./scripts/check-coverage.sh --update  # Update thresholds to current values (ratchet up only)
set -euo pipefail

THRESHOLDS_FILE="coverage-thresholds.json"

# Files excluded from coverage: generated code (sqlc, protobuf) and thin
# wrappers over external dependencies (Redis, PostgreSQL) that are covered
# by e2e tests rather than unit tests. Keeping these in the weighted total
# drowns out real signal from the hand-written, unit-testable code.
COVERAGE_EXCLUDES='(\.gen\.go|/cache/redis\.go|/pubsub/redis\.go|/config/store_pg\.go|/schema/store_pg\.go):'

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

declare -A COVERAGES
declare -A FAIL_REASONS
declare -A FAIL_LOGS

# run_coverage MODULE DIR PATTERN
# Populates COVERAGES[MODULE] with a percentage, or "FAIL" if tests failed or
# no coverage was produced. On FAIL, records FAIL_REASONS[MODULE] and captures
# the tail of the combined test output in FAIL_LOGS[MODULE].
run_coverage() {
  local module=$1
  local dir=$2
  local pattern=$3
  local log_file cov_file
  log_file=$(mktemp)
  cov_file=$(mktemp)

  # `go tool cover -func` resolves package paths relative to the current module,
  # so it must run in the same directory as `go test`.
  local test_exit=0
  local cov=""
  (
    if [ "$dir" != "." ]; then cd "$dir"; fi
    go test "$pattern" -coverprofile="$cov_file" -count=1 >"$log_file" 2>&1 || exit $?
    [ -s "$cov_file" ] || exit 64  # distinguishable from go test exits
    # Strip excluded files (generated code + external-dep wrappers) from the
    # profile before computing the total. grep -v returning 1 means nothing
    # matched; that's fine (nothing to exclude), so don't let set -e abort.
    grep -vE "$COVERAGE_EXCLUDES" "$cov_file" > "$cov_file.filtered" || true
    if [ -s "$cov_file.filtered" ]; then
      mv "$cov_file.filtered" "$cov_file"
    else
      rm -f "$cov_file.filtered"
    fi
    go tool cover -func="$cov_file" 2>/dev/null \
      | awk '/^total:/ {gsub(/%/,""); print $NF}' > "$log_file.cov"
  ) || test_exit=$?

  if [ "$test_exit" -eq 64 ]; then
    COVERAGES[$module]="FAIL"
    FAIL_REASONS[$module]="no coverage produced (coverprofile empty)"
    FAIL_LOGS[$module]=$(tail -n 40 "$log_file")
  elif [ "$test_exit" -ne 0 ]; then
    COVERAGES[$module]="FAIL"
    FAIL_REASONS[$module]="tests failed (exit $test_exit)"
    FAIL_LOGS[$module]=$(tail -n 40 "$log_file")
  else
    cov=$(cat "$log_file.cov" 2>/dev/null || true)
    if [ -z "$cov" ]; then
      COVERAGES[$module]="FAIL"
      FAIL_REASONS[$module]="coverage total not parseable"
      FAIL_LOGS[$module]=$(tail -n 40 "$log_file")
    else
      COVERAGES[$module]=$cov
    fi
  fi
  rm -f "$log_file" "$log_file.cov" "$cov_file"
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

print_fail_logs() {
  local stream=$1
  for module in $(echo "${!MODULES[@]}" | tr ' ' '\n' | sort); do
    if [ "${COVERAGES[$module]:-}" = "FAIL" ] && [ -n "${FAIL_LOGS[$module]:-}" ]; then
      if [ "$stream" = "stderr" ]; then
        echo "--- $module test output (last 40 lines) ---" >&2
        echo "${FAIL_LOGS[$module]}" >&2
        echo "" >&2
      else
        echo "--- $module test output (last 40 lines) ---"
        echo "${FAIL_LOGS[$module]}"
        echo ""
      fi
    fi
  done
}

# Collect all coverage values.
for module in "${!MODULES[@]}"; do
  run_coverage "$module" "${MODULE_DIRS[$module]}" "${MODULES[$module]}"
done

if [ "${1:-}" = "--update" ]; then
  # A failing module cannot be ratcheted — its coverage is unknown. Exit non-zero
  # rather than silently writing 0 or preserving a stale threshold.
  any_fail=0
  for module in $(echo "${!MODULES[@]}" | tr ' ' '\n' | sort); do
    if [ "${COVERAGES[$module]}" = "FAIL" ]; then
      echo "ERROR: $module: ${FAIL_REASONS[$module]}" >&2
      any_fail=1
    fi
  done
  if [ "$any_fail" -eq 1 ]; then
    echo "" >&2
    print_fail_logs stderr
    echo "Cannot update thresholds while modules are failing." >&2
    exit 1
  fi

  echo "{"
  first=true
  for module in $(echo "${!MODULES[@]}" | tr ' ' '\n' | sort); do
    current=${COVERAGES[$module]}
    old=$(get_threshold "$module")
    new=$(python3 -c "print(max(float('$current'), float('$old')))")
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
  if [ "$current" = "FAIL" ]; then
    printf "%-25s %7s   ✗ TESTS FAILED (%s)\n" "$module" "--" "${FAIL_REASONS[$module]}"
    failed=1
    continue
  fi
  if python3 -c "exit(0 if float('$current') >= float('$threshold') else 1)" 2>/dev/null; then
    printf "%-25s %6s%% (threshold: %s%%) ✓\n" "$module" "$current" "$threshold"
  else
    printf "%-25s %6s%% (threshold: %s%%) ✗ BELOW THRESHOLD\n" "$module" "$current" "$threshold"
    failed=1
  fi
done

if [ "$failed" -eq 1 ]; then
  echo ""
  print_fail_logs stdout
  echo "Fix the failing modules or update thresholds with:"
  echo "  ./scripts/check-coverage.sh --update > coverage-thresholds.json"
  exit 1
fi

echo ""
echo "All modules meet coverage thresholds."
