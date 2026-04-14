# REST/HTTP Gateway

**Status:** Complete
**Started:** 2026-04-09

---

## Goal

Expose the entire gRPC API as REST/JSON via grpc-gateway. Foundation for the admin GUI, non-Go SDKs, and curl-based usage.

## What Was Done

### Proto Changes
- [x] Added `buf.build/googleapis/googleapis` dependency to `buf.yaml`
- [x] Added `import "google/api/annotations.proto"` to all 4 service protos
- [x] Added `google.api.http` annotations to all 32 RPCs

### Build Tooling
- [x] Added `protoc-gen-grpc-gateway` v2.27.3 to `buf.gen.yaml` and `build/Dockerfile.tools`
- [x] Added `protoc-gen-openapiv2` v2.27.3 with merged output
- [x] Disabled `go_package_prefix` for googleapis module (prevents broken imports)

### Server Integration
- [x] `internal/server/gateway.go` — HTTP reverse proxy with auth header forwarding
- [x] Gateway is **opt-in**: only starts when `HTTP_PORT` env var is set
- [x] Auth headers (x-subject, x-role, x-tenant-id, authorization) forwarded from HTTP to gRPC metadata
- [x] Wired into `cmd/server/main.go` with graceful shutdown

### Tests
- [x] Unit tests: gateway creation, disable when no port, auth header forwarding
- [x] E2E tests: REST version, schema lifecycle CRUD, auth headers, 404 mapping

### Outputs
- [x] Gateway `.pb.gw.go` files for all 4 services
- [x] Merged OpenAPI spec at `docs/api/openapi.swagger.json`
- [x] Docker Compose updated with HTTP_PORT=8080, port 8080 mapped

## Key Decisions
- Gateway is optional — `HTTP_PORT=""` (default) means gRPC only
- Embedded in server binary, not a sidecar
- Field paths in URLs: `field_path` as path segment for GetField/SetField
- Subscribe streams as newline-delimited JSON via grpc-gateway default
- OpenAPI spec merged into single file
