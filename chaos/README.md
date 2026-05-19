# Chaos test suite

End-to-end fault-injection tests for the decree server. Covers the failure modes
required for Beta Readiness (issue [#469](https://github.com/opendecree/decree/issues/469)).

## What it tests

| Test | Scenario | Assertions |
|------|----------|------------|
| `TestDBOutage_MidFlight` | Pause postgres mid-flight | Writes return `codes.Internal` or `codes.DeadlineExceeded` (not raw EOF); server recovers on unpause |
| `TestDBRecovery_AfterRestart` | Full postgres stop/start | Same error assertions; pgxpool reconnects automatically within 60s |
| `TestRedisOutage_CacheFallback` | Pause Redis | `GetAll` returns correct values via DB fallback; `SetField` succeeds (publish warn-only) |
| `TestRedisOutage_PubSubDegraded` | Pause Redis | `Subscribe` returns `codes.Internal`; server doesn't crash; `Subscribe` recovers within 15s |
| `TestGracefulShutdown_UnderLoad` | SIGTERM under 1000 concurrent RPCs | All goroutines terminate; shutdown < 30s; no `codes.Unknown` (dropped requests) |

## Running locally

```bash
# From decree/ root — brings up docker stack, runs tests, tears down.
make chaos
```

The stack must not already be running when you call `make chaos` (it will call
`docker compose up -d` internally). If you want to run against a pre-existing
stack, set `SERVICE_ADDR` and run the tests directly:

```bash
cd chaos
SERVICE_ADDR=localhost:9090 go test -v -tags=chaos -timeout=300s ./...
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_ADDR` | `localhost:9090` | gRPC address of the decree server |
| `POSTGRES_CONTAINER` | `decree-postgres-1` | Docker container name for postgres |
| `REDIS_CONTAINER` | `decree-redis-1` | Docker container name for Redis |
| `SERVER_CONTAINER` | `decree-service-1` | Docker container name for the decree server |

Container names follow Docker Compose conventions: `<project>-<service>-<n>`.
If your compose project name differs from `decree`, set the env vars accordingly.

## Running in CI

The chaos suite runs on a weekly schedule and on manual dispatch (not on every PR —
these tests kill containers and take ~5 minutes).

```bash
# Trigger manually:
gh workflow run chaos.yml --repo opendecree/decree
```

CI uses the same `make chaos` target. Container names use the default `decree-*`
compose project prefix.

## Design notes

- **No new dependencies.** Container control uses `exec.Command("docker", ...)`.
- **Build tag `chaos`** keeps these tests out of `make test` and `make e2e`.
- **Cleanup ordering.** `t.Cleanup` uses LIFO — the server restart cleanup is
  registered last in `TestGracefulShutdown_UnderLoad` so it runs first, before
  schema/tenant deletions that require a live server.
- **Pause vs stop.** `docker pause` (SIGSTOP) simulates a network partition without
  container restart overhead. `docker stop` is used only in `TestDBRecovery_AfterRestart`
  to test the full reconnect path.
