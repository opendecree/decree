# Multi-Language SDKs

**Status:** Planning
**Started:** 2026-04-09

---

## Goal

TypeScript and Python SDKs so non-Go users get a good experience from day one. Generated gRPC stubs + thin ergonomic wrappers.

## Repo Strategy

Separate repos per language, proto published to BSR:
- `opendecree/decree-sdk-ts` — TypeScript/Node.js
- `opendecree/decree-sdk-python` — Python
- Proto source of truth stays in `opendecree/decree`
- Published to BSR: `buf.build/opendecree/decree`
- SDK repos generate from BSR, not from local proto files

### BSR Publishing (main repo)
- [ ] Add `buf push --tag $TAG` to release workflow
- [ ] Verify module appears on `buf.build/opendecree/decree`

## TypeScript SDK (`opendecree/decree-sdk-ts`)

**Stack:** `@connectrpc/connect` + `@bufbuild/protobuf` (buf ecosystem, gRPC + HTTP)
**Package:** `@opendecree/sdk` on npm

### Wrapper API
- `ConfigClient` — get, getAll, set, setMany, setNull + typed (getInt, getBool, getFloat)
- `AdminClient` — createSchema, getSchema, listSchemas, publishSchema, createTenant, getTenant, listTenants, lockField, unlockField
- Auth interceptor (subject, role, tenantId, bearerToken)
- Error mapping (ConnectError codes → NotFoundError, LockedError, AlreadyExistsError, etc.)

### Work Items
- [ ] Scaffold repo with package.json, tsconfig.json
- [ ] `buf.gen.yaml` pulling from BSR
- [ ] Generate stubs into `src/gen/`
- [ ] Implement `ConfigClient` wrapper
- [ ] Implement `AdminClient` wrapper
- [ ] Auth interceptor
- [ ] Error mapping
- [ ] Tests (vitest with mocked Connect transport)
- [ ] README with install + usage examples
- [ ] CI (GitHub Actions: lint, test, build)
- [ ] npm publish setup

## Python SDK (`opendecree/decree-sdk-python`)

**Stack:** `grpcio` + `grpcio-tools` + `mypy-protobuf`
**Package:** `opendecree` on PyPI, Python 3.10+

### Wrapper API
- `ConfigClient` — get, get_all, set, set_many, set_null + typed (get_int, get_bool, get_float)
- `AdminClient` — create_schema, get_schema, list_schemas, publish_schema, create_tenant, get_tenant, list_tenants, lock_field, unlock_field
- Auth metadata injection
- Error mapping (grpc.StatusCode → NotFoundError, LockedError, etc.)

### Work Items
- [ ] Scaffold repo with pyproject.toml
- [ ] Generate stubs from BSR
- [ ] Implement `ConfigClient` wrapper
- [ ] Implement `AdminClient` wrapper
- [ ] Auth helper
- [ ] Error mapping
- [ ] Tests (pytest with grpcio-testing)
- [ ] README with install + usage examples
- [ ] CI (GitHub Actions: lint, test, type check)
- [ ] PyPI publish setup

## Key Decisions
- Separate repos (independent release cycles, CI, package managers)
- Proto via BSR (decouples from main repo file structure)
- Thin wrappers only — not full Go SDK parity (no snapshots, CAS, watcher)
- Connect for TS (supports both gRPC and HTTP transport)
- Official grpcio for Python (most widely adopted)
