#!/usr/bin/env bash
# Calculate weighted test coverage for the decree server, excluding
# infrastructure code that is tested at the integration/e2e level.
#
# Excluded files:
#   store_pg.go      — PostgreSQL store implementations (thin DB wrappers, tested via e2e)
#   redis.go         — Redis cache/pubsub implementations (thin wrappers, tested via e2e)
#   dbstore/         — sqlc-generated query code
#   storage/postgres.go — interface definitions only
#   telemetry/       — OpenTelemetry provider wiring (boilerplate)
#
# Usage:
#   ./scripts/coverage.sh          # print coverage percentage
#   ./scripts/coverage.sh -v       # verbose: show per-function breakdown

set -euo pipefail

COVER_OUT=$(mktemp)
COVER_FILTERED=$(mktemp)
trap 'rm -f "$COVER_OUT" "$COVER_FILTERED"' EXIT

# Run tests with coverage profile.
go test -coverprofile="$COVER_OUT" -count=1 ./internal/... > /dev/null 2>&1

# Filter out infrastructure files.
head -1 "$COVER_OUT" > "$COVER_FILTERED"
grep -v "store_pg.go" "$COVER_OUT" \
  | grep -v "redis.go" \
  | grep -v "dbstore/" \
  | grep -v "storage/postgres.go" \
  | grep -v "telemetry/" \
  | grep -v "^mode:" >> "$COVER_FILTERED"

if [[ "${1:-}" == "-v" ]]; then
  go tool cover -func="$COVER_FILTERED"
else
  go tool cover -func="$COVER_FILTERED" | tail -1 | awk '{print $NF}'
fi
