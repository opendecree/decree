#!/usr/bin/env bash
# strip-replaces.sh — prepare a release commit so published modules are go-installable.
#
# WHY: committed `replace github.com/opendecree/decree/...` directives make
# `go install <module>/cmd/...@<ver>` and `go get <module>@<ver>` impossible —
# Go refuses any module whose go.mod contains replace directives. The directives
# exist for local development (see ADR-006); they must NOT appear in a tagged,
# published commit. See ADR-007.
#
# This script does not touch `main`. It runs against the detached release commit
# (after the require-version bump PR is merged — see RELEASING.md), strips the
# internal replace directives, and regenerates go.sum via `go mod tidy` so the
# tagged tree carries the internal-module hashes.
#
# Usage:
#   strip-replaces.sh strip [module ...]   # remove internal replaces (text only; no network)
#   strip-replaces.sh tidy  [module ...]   # go mod tidy (deps must already be published/tagged)
#   strip-replaces.sh check [module ...]   # exit non-zero if any module still has internal replaces
#
# With no module args, operates on every published module (all modules except the
# internal-only test/example modules listed in EXCLUDE_RE). `tidy` runs leaf-first,
# so each module is tidied only after its intra-repo deps are available on the proxy.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

# Internal-only modules — never published, keep their replace directives.
EXCLUDE_RE='^(examples/|e2e|chaos|stress|fixtures/)'

# Leaf-first dependency order for `go mod tidy` (deps before dependents).
LEAF_FIRST_ORDER='api sdk/retry sdk/configclient sdk/configwatcher sdk/adminclient sdk/grpctransport sdk/tools sdk/contrib/envconfig sdk/contrib/koanf sdk/contrib/viper contrib/decree-docs cmd/decree .'

published_modules() {
  # Every go.mod dir, minus the internal-only modules, emitted leaf-first.
  local all
  all="$(find . -name go.mod -printf '%h\n' | sed 's#^\./##' | sort)"
  for m in $LEAF_FIRST_ORDER; do
    if printf '%s\n' "$all" | grep -qx "$m" && ! printf '%s\n' "$m" | grep -qE "$EXCLUDE_RE"; then
      echo "$m"
    fi
  done
}

internal_replace_count() {
  grep -cE '^replace github.com/opendecree/decree' "$1/go.mod" 2>/dev/null || true
}

mods() { if [ "$#" -gt 0 ]; then printf '%s\n' "$@"; else published_modules; fi; }

cmd="${1:-}"; shift || true
case "$cmd" in
  strip)
    while read -r m; do
      [ -n "$m" ] || continue
      n="$(internal_replace_count "$m")"
      if [ "${n:-0}" -gt 0 ]; then
        sed -i '/^replace github.com\/opendecree\/decree/d' "$m/go.mod"
        echo "stripped $n internal replace(s): $m/go.mod"
      fi
    done < <(mods "$@")
    ;;
  tidy)
    while read -r m; do
      [ -n "$m" ] || continue
      echo "go mod tidy: $m"
      (cd "$m" && GOWORK=off go mod tidy)
    done < <(mods "$@")
    ;;
  check)
    bad=0
    while read -r m; do
      [ -n "$m" ] || continue
      n="$(internal_replace_count "$m")"
      if [ "${n:-0}" -gt 0 ]; then
        echo "FAIL: $m/go.mod still has $n internal replace directive(s)"
        bad=1
      fi
    done < <(mods "$@")
    if [ "$bad" -eq 0 ]; then echo "OK: no internal replace directives in published modules"; fi
    exit "$bad"
    ;;
  *)
    echo "usage: strip-replaces.sh {strip|tidy|check} [module ...]" >&2
    exit 2
    ;;
esac
