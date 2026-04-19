#!/usr/bin/env bash
# e2e-coverage.sh — Run e2e tests against a coverage-instrumented server
# and report coverage on the files excluded from the unit-test total
# (generated code + Redis/PG wrappers). This closes the loop on the
# check-coverage.sh exclusion list: the excluded files must be exercised
# by e2e, or they are silently untested.
#
# Usage: ./scripts/e2e-coverage.sh
#
# Exit codes:
#   0 — e2e passed and the report is produced
#   1 — e2e failed, a prerequisite is missing, or coverage collection broke
set -euo pipefail

# Must match check-coverage.sh's COVERAGE_EXCLUDES.
INCLUDE_REGEX='(\.gen\.go|/cache/redis\.go|/pubsub/redis\.go|/config/store_pg\.go|/schema/store_pg\.go):'

COV_DIR=$(mktemp -d)
BIN=$(mktemp -u)
PROFILE=$(mktemp)
trap 'rm -rf "$COV_DIR" "$BIN" "$PROFILE"; docker compose down -v >/dev/null 2>&1 || true' EXIT

echo "=== Build coverage-instrumented server binary ==="
# -coverpkg instruments all packages in the main module (including internal/)
# so we can observe store_pg.go, redis.go, dbstore/*.gen.go, etc.
go build -cover -covermode=atomic -coverpkg=./... -o "$BIN" ./cmd/server

echo "=== Start postgres + redis (service runs locally, not in docker) ==="
# --wait rejects one-shot services (migrate exits 0 which !== "healthy"),
# so start the long-running services first, then run migrate as a one-shot.
docker compose up -d --wait postgres redis >/dev/null
docker compose run --rm migrate >/dev/null

echo "=== Start server with GOCOVERDIR=$COV_DIR ==="
GOCOVERDIR="$COV_DIR" \
  DB_WRITE_URL="postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable" \
  REDIS_URL="redis://localhost:6379" \
  GRPC_PORT=9090 \
  HTTP_PORT=8080 \
  USAGE_FLUSH_INTERVAL=1s \
  "$BIN" &
SERVER_PID=$!
trap 'kill -TERM '"$SERVER_PID"' 2>/dev/null || true; rm -rf "$COV_DIR" "$BIN" "$PROFILE"; docker compose down -v >/dev/null 2>&1 || true' EXIT

# Wait for server to accept connections on 9090.
for _ in $(seq 1 30); do
  if (echo > /dev/tcp/localhost/9090) >/dev/null 2>&1; then break; fi
  sleep 0.5
done

echo "=== Run e2e tests ==="
e2e_exit=0
(cd e2e && go test -tags=e2e -count=1 ./...) || e2e_exit=$?

echo "=== Shut down server (flushes coverage) ==="
kill -TERM "$SERVER_PID" 2>/dev/null || true
wait "$SERVER_PID" 2>/dev/null || true

if [ "$e2e_exit" -ne 0 ]; then
  echo "ERROR: e2e tests failed (exit $e2e_exit)" >&2
  exit 1
fi

echo "=== Convert coverage data to text profile ==="
go tool covdata textfmt -i="$COV_DIR" -o="$PROFILE"

# Keep only the lines we care about (excluded-from-unit files).
FILTERED=$(mktemp)
head -1 "$PROFILE" > "$FILTERED"
grep -E "$INCLUDE_REGEX" "$PROFILE" >> "$FILTERED" || true

if [ "$(wc -l < "$FILTERED")" -le 1 ]; then
  echo "WARNING: no coverage data captured for excluded files — did the server actually run?" >&2
  exit 1
fi

echo ""
echo "=== E2E coverage on files excluded from unit-test total ==="

# Per-file summary: sum statements + hits per file, compute percentage.
# Much more useful than the function list when there are many generated files.
tail -n +2 "$FILTERED" | awk '
{
  # coverprofile line: file:start.col,end.col stmts hits
  split($1, a, ":")
  file = a[1]
  stmts[file] += $2
  if ($3 > 0) covered[file] += $2
}
END {
  for (f in stmts) {
    pct = (stmts[f] == 0) ? 0 : covered[f] / stmts[f] * 100
    printf "%7.1f%%  %6d stmts  %s\n", pct, stmts[f], f
  }
}' | sort -rn

echo ""
go tool cover -func="$FILTERED" | awk '/^total:/ {print "E2E coverage of excluded files: " $NF}'

# Flag files that the exclusion hides from unit coverage but e2e also misses.
uncovered=$(tail -n +2 "$FILTERED" | awk '
{
  split($1, a, ":")
  file = a[1]
  stmts[file] += $2
  if ($3 > 0) covered[file] += $2
}
END {
  for (f in stmts) {
    pct = (stmts[f] == 0) ? 0 : covered[f] / stmts[f] * 100
    if (pct < 50) print f, pct
  }
}')
if [ -n "$uncovered" ]; then
  echo ""
  echo "WARNING: excluded files with <50% e2e coverage — exclusion is hiding dead code:" >&2
  echo "$uncovered" >&2
fi
rm -f "$FILTERED"
