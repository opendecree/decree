#!/usr/bin/env bash
# check-coverage.sh — Enforce per-module coverage thresholds (ratchet).
#
# Usage:
#   ./scripts/check-coverage.sh                            # Run tests + check thresholds
#   ./scripts/check-coverage.sh --profile mod=path ...    # Check using pre-computed profiles
#   ./scripts/check-coverage.sh --update                   # Update thresholds (always runs tests)
#
# --profile can be repeated once per module. Paths are relative to the
# directory from which the script is invoked (typically the repo root).
# Modules with no --profile arg fall back to running go test.
set -euo pipefail

THRESHOLDS_FILE="coverage-thresholds.json"

# Canonical exclude list for coverage tooling.
# codecov.yml ignore section must mirror these decisions (see comment there).
#
# Excluded categories and rationale:
#   *.gen.go                     — sqlc/protobuf generated code, not hand-written
#   /cache/redis.go              — thin Redis wrapper, covered by e2e only
#   /pubsub/redis.go             — thin Redis wrapper, covered by e2e only
#   /audit/store_pg.go           — thin PG store wrapper, covered by e2e only
#   /config/store_pg.go          — thin PG store wrapper, covered by e2e only
#   /schema/store_pg.go          — thin PG store wrapper, covered by e2e only
#
# Intentionally NOT excluded:
#   telemetry/                   — counted by both ratchet and Codecov (boilerplate, but measurable)
#   cmd/decree                   — 86%+ coverage, tracked in ratchet and Codecov
#   sdk/grpctransport            — ratchet floor 8%; excluded from Codecov patch/project (defensible)
COVERAGE_EXCLUDES='(\.gen\.go|/cache/redis\.go|/pubsub/redis\.go|/audit/store_pg\.go|/config/store_pg\.go|/schema/store_pg\.go):'

# Module → test pattern pairs.
declare -A MODULES=(
  ["internal"]="./internal/..."
  ["sdk/configclient"]="./..."
  ["sdk/adminclient"]="./..."
  ["sdk/configwatcher"]="./..."
  ["sdk/tools"]="./..."
  ["contrib/decree-docs"]="./..."
  ["cmd/decree"]="./..."
)

# Module → working directory (empty = repo root).
declare -A MODULE_DIRS=(
  ["internal"]="."
  ["sdk/configclient"]="sdk/configclient"
  ["sdk/adminclient"]="sdk/adminclient"
  ["sdk/configwatcher"]="sdk/configwatcher"
  ["sdk/tools"]="sdk/tools"
  ["contrib/decree-docs"]="contrib/decree-docs"
  ["cmd/decree"]="cmd/decree"
)

declare -A COVERAGES
declare -A FAIL_REASONS
declare -A FAIL_LOGS

# Parse arguments.
UPDATE=false
declare -A PROFILES=()

while [[ $# -gt 0 ]]; do
  case $1 in
    --update)
      UPDATE=true
      shift
      ;;
    --profile)
      [[ $# -lt 2 ]] && { echo "--profile requires an argument (module=path)" >&2; exit 1; }
      IFS='=' read -r _mod _path <<< "$2"
      [[ -z "${_mod:-}" || -z "${_path:-}" ]] && { echo "--profile value must be module=path" >&2; exit 1; }
      PROFILES[$_mod]=$_path
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

# run_coverage MODULE DIR PATTERN
# Runs go test and populates COVERAGES[MODULE], or "FAIL" on error.
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

# read_coverage MODULE DIR PROFILE_PATH
# Computes coverage from a pre-existing profile (no go test). Populates
# COVERAGES[MODULE] the same way as run_coverage, but without FAIL_LOGS.
read_coverage() {
  local module=$1
  local dir=$2
  local profile_src=$3

  # Resolve to absolute path before any cd.
  local abs_profile
  abs_profile=$(realpath "$profile_src" 2>/dev/null) || {
    COVERAGES[$module]="FAIL"
    FAIL_REASONS[$module]="profile not found: $profile_src"
    return
  }

  if [ ! -s "$abs_profile" ]; then
    COVERAGES[$module]="FAIL"
    FAIL_REASONS[$module]="profile empty or missing: $profile_src"
    return
  fi

  local cov_file
  cov_file=$(mktemp)
  grep -vE "$COVERAGE_EXCLUDES" "$abs_profile" > "$cov_file" || true
  [ -s "$cov_file" ] || cp "$abs_profile" "$cov_file"

  local cov
  cov=$(
    if [ "$dir" != "." ]; then cd "$dir"; fi
    go tool cover -func="$cov_file" 2>/dev/null | awk '/^total:/ {gsub(/%/,""); print $NF}'
  )

  rm -f "$cov_file"

  if [ -z "$cov" ]; then
    COVERAGES[$module]="FAIL"
    FAIL_REASONS[$module]="coverage total not parseable from profile $profile_src"
  else
    COVERAGES[$module]=$cov
  fi
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
  if [ "$UPDATE" = false ] && [ -n "${PROFILES[$module]:-}" ]; then
    read_coverage "$module" "${MODULE_DIRS[$module]}" "${PROFILES[$module]}"
  else
    run_coverage "$module" "${MODULE_DIRS[$module]}" "${MODULES[$module]}"
  fi
done

if [ "$UPDATE" = true ]; then
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
