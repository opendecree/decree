#!/usr/bin/env bash
# Upgrade test: old binary → goose up → new binary on same DB.
#
# Phases:
#   1. Find previous release tag and pull its server image.
#   2. Extract previous migrations from git.
#   3. Start postgres + redis.
#   4. Apply old migrations (goose).
#   5. Start old server → TestUpgrade_Populate.
#   6. Apply new migrations (goose up).
#   7. Start new server → TestUpgrade_Assert.
#
# Env vars:
#   TOOLS_IMAGE   goose image (default: decree-tools)
#   SERVICE_IMAGE new server image (default: decree-service)
#   GH_TOKEN      required for gh release list
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TOOLS_IMAGE="${TOOLS_IMAGE:-decree-tools}"
SERVICE_IMAGE="${SERVICE_IMAGE:-decree-service}"
NETWORK="decree-upgrade"
PG_CONTAINER="decree-upgrade-pg"
REDIS_CONTAINER="decree-upgrade-redis"
OLD_SERVER="decree-upgrade-old"
NEW_SERVER="decree-upgrade-new"
DB_URL="postgres://centralconfig:localdev@${PG_CONTAINER}:5432/centralconfig?sslmode=disable"
TMP_DIR=""

cleanup() {
  docker rm -f "$PG_CONTAINER" "$REDIS_CONTAINER" "$OLD_SERVER" "$NEW_SERVER" 2>/dev/null || true
  docker network rm "$NETWORK" 2>/dev/null || true
  [ -n "$TMP_DIR" ] && rm -rf "$TMP_DIR"
}
trap cleanup EXIT

wait_tcp() {
  local host="$1" port="$2" label="$3" retries="${4:-30}"
  echo "Waiting for $label on $host:$port ..."
  for _ in $(seq 1 "$retries"); do
    if nc -z "$host" "$port" 2>/dev/null; then
      echo "$label is ready."
      return 0
    fi
    sleep 1
  done
  echo "ERROR: $label did not become ready after ${retries}s" >&2
  return 1
}

# ── 1. Find previous release tag ──────────────────────────────────────────────
PREV_TAG=$(gh release list --repo opendecree/decree --limit 20 --json tagName,isDraft \
  --jq '[.[] | select(.isDraft == false)] | .[0].tagName' 2>/dev/null || true)

if [ -z "$PREV_TAG" ]; then
  echo "No previous release found — skipping upgrade test."
  exit 0
fi
echo "Previous release: $PREV_TAG"

# ── 2. Extract previous migrations from git ───────────────────────────────────
TMP_DIR=$(mktemp -d)
OLD_MIGRATIONS_DIR="$TMP_DIR/migrations"
mkdir -p "$OLD_MIGRATIONS_DIR"

while IFS= read -r f; do
  [ -z "$f" ] && continue
  fname="${f##*/}"
  git show "${PREV_TAG}:${f}" > "${OLD_MIGRATIONS_DIR}/${fname}"
done < <(git ls-tree --name-only "$PREV_TAG" db/migrations/ 2>/dev/null || true)

if [ -z "$(ls -A "$OLD_MIGRATIONS_DIR" 2>/dev/null)" ]; then
  echo "No migrations in $PREV_TAG — skipping upgrade test."
  exit 0
fi
echo "Old migrations: $(ls "$OLD_MIGRATIONS_DIR" | tr '\n' ' ')"

# ── 3. Pull previous server image ─────────────────────────────────────────────
PREV_IMAGE="ghcr.io/opendecree/decree:${PREV_TAG}"
if ! docker pull "$PREV_IMAGE"; then
  echo "Image $PREV_IMAGE not available — skipping upgrade test."
  exit 0
fi

# ── 4. Start postgres + redis ─────────────────────────────────────────────────
docker network create "$NETWORK"

docker run -d --name "$PG_CONTAINER" --network "$NETWORK" \
  -p 5499:5432 \
  -e POSTGRES_DB=centralconfig \
  -e POSTGRES_USER=centralconfig \
  -e POSTGRES_PASSWORD=localdev \
  postgres:17

docker run -d --name "$REDIS_CONTAINER" --network "$NETWORK" \
  redis:7 redis-server --maxmemory 128mb --maxmemory-policy allkeys-lru

wait_tcp 127.0.0.1 5499 "postgres"
until docker exec "$PG_CONTAINER" pg_isready -U centralconfig 2>/dev/null; do sleep 1; done

# ── 5. Apply old migrations ───────────────────────────────────────────────────
echo "--- applying old migrations ---"
docker run --rm --network "$NETWORK" \
  -v "${OLD_MIGRATIONS_DIR}:/migrations:ro" \
  "$TOOLS_IMAGE" \
  goose -dir /migrations postgres "$DB_URL" up

# ── 6. Start old server, populate fixtures ────────────────────────────────────
echo "--- starting old server ---"
docker run -d --name "$OLD_SERVER" --network "$NETWORK" \
  -p 19090:9090 \
  -e GRPC_PORT=9090 \
  -e DB_WRITE_URL="$DB_URL" \
  -e DB_READ_URL="$DB_URL" \
  -e REDIS_URL="redis://${REDIS_CONTAINER}:6379" \
  -e ENABLE_SERVICES=schema,config,audit \
  -e HTTP_PORT=8080 \
  -e INSECURE_LISTEN=1 \
  -e ENABLE_REFLECTION=1 \
  -e SCHEMA_MAX_FIELDS=100 \
  -e SCHEMA_MAX_DOC_BYTES=4096 \
  "$PREV_IMAGE"

wait_tcp 127.0.0.1 19090 "old server"

echo "--- populating fixtures ---"
(cd "$REPO_ROOT/e2e" && SERVICE_ADDR=localhost:19090 \
  go test -tags=e2e,upgrade -run TestUpgrade_Populate -v -count=1 ./...)

docker stop "$OLD_SERVER"
docker rm "$OLD_SERVER"

# ── 7. Apply new migrations ───────────────────────────────────────────────────
echo "--- applying new migrations ---"
docker run --rm --network "$NETWORK" \
  -v "${REPO_ROOT}/db/migrations:/migrations:ro" \
  "$TOOLS_IMAGE" \
  goose -dir /migrations postgres "$DB_URL" up

# ── 8. Start new server, assert data integrity ───────────────────────────────
echo "--- starting new server ---"
docker run -d --name "$NEW_SERVER" --network "$NETWORK" \
  -p 29090:9090 \
  -e GRPC_PORT=9090 \
  -e DB_WRITE_URL="$DB_URL" \
  -e DB_READ_URL="$DB_URL" \
  -e REDIS_URL="redis://${REDIS_CONTAINER}:6379" \
  -e ENABLE_SERVICES=schema,config,audit \
  -e HTTP_PORT=8080 \
  -e INSECURE_LISTEN=1 \
  -e ENABLE_REFLECTION=1 \
  -e SCHEMA_MAX_FIELDS=100 \
  -e SCHEMA_MAX_DOC_BYTES=4096 \
  "$SERVICE_IMAGE"

wait_tcp 127.0.0.1 29090 "new server"

echo "--- asserting data integrity ---"
(cd "$REPO_ROOT/e2e" && SERVICE_ADDR=localhost:29090 \
  go test -tags=e2e,upgrade -run TestUpgrade_Assert -v -count=1 ./...)

echo "--- upgrade test passed ---"
