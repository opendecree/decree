# Coding Guidelines

Shared philosophy and per-SDK conventions for OpenDecree contributors.

## Shared Philosophy

### Vanilla Principle

Prefer standard library and widely-adopted tools. Avoid niche dependencies. When an external
dependency is required (database driver, gRPC runtime), wrap it behind an interface so it can
be replaced without touching call sites.

### Minimal Dependencies

SDK packages are consumed by users alongside their own dependency trees. Every added dependency
is a potential version conflict. Add a runtime dependency only when the alternative is
reimplementing a non-trivial protocol (gRPC, Protobuf). Everything else: standard library.

### No Build Magic

Code generation is acceptable when the output is committed and reproducible. Avoid
source-transforming build steps, unconventional preprocessors, or anything that makes
`go build` / `pip install` / `npm ci` non-standard.

---

## Go

Applies to the server, CLI, and all SDK modules under `sdk/`.

### Standard Library First

Reach for the standard library before adding a dependency. For HTTP clients, JSON, logging,
sync primitives, and testing in SDK modules — the standard library is the answer.

### Testing

- **Server** (`go.mod` at repo root): testify is permitted.
- **SDK modules** (`sdk/*/go.mod`): use `testing` + `errors.Is` only. No testify. Keeps SDK
  consumers free of the transitive dependency.

### Go Version Policy

| Module | Minimum Go |
|--------|-----------|
| Server (root module) | 1.25 |
| CLI (`cmd/decree`) | 1.24 |
| `api`, `sdk/grpctransport`, `sdk/tools` | 1.24 |
| SDK core (`configclient`, `adminclient`, `configwatcher`) | 1.22 |

Write SDK core code against the 1.22 feature set. Do not use language or stdlib additions
from later versions in those modules.

### Lint and Format

All code must pass `make lint`, which runs `golangci-lint run` and `gofmt --diff`. CI fails
on any formatting difference. Run locally before committing.

### Functional Options

Required arguments are positional. Optional configuration uses `With...()` functional options.
Do not make required fields optional by giving them a zero-value default.

---

## Python SDK

Repo: [decree-python](https://github.com/opendecree/decree-python)

### Runtime Dependencies

Runtime deps are: `grpcio`, `protobuf`, `googleapis-common-protos`. That is the complete list.
OpenTelemetry instrumentation is an optional extra (`pip install opendecree[otel]`).

Do not add runtime dependencies. If a feature genuinely requires one, open a discussion first.

### Type Annotations

All public functions and methods must have complete type annotations. Run `mypy` (`make lint`)
before committing. `mypy-protobuf` generates stubs for generated proto code.

### Asyncio

The SDK ships both sync and async clients. New features must be implemented in both. Prefer
`asyncio`-native patterns (`async def`, `async for`, `async with`). Do not use `asyncio.run()`
inside library code.

### Style

`ruff` handles formatting and linting. Run `make lint` before committing. No manual style
fixes — let the tool enforce it.

---

## TypeScript SDK

Repo: [decree-typescript](https://github.com/opendecree/decree-typescript)

### Runtime Dependencies

The only runtime dependency is `@grpc/grpc-js`. Keep it that way.

### Strict TypeScript

The SDK uses strict mode plus additional checks:

```json
"noUncheckedIndexedAccess": true,
"exactOptionalPropertyTypes": true,
"noImplicitOverride": true,
"noFallthroughCasesInSwitch": true
```

Do not weaken these settings. Do not use `any` or non-null assertions (`!`) without a
comment explaining why the type system cannot express the invariant.

### No Transpiler Magic

Target ES2022 with `module: "Node16"`. No Babel, no esbuild transforms, no custom loaders.
What TypeScript emits is what ships.

### Lint and Format

Biome handles both lint and format. Run `npx biome check --write src/ test/` before committing.
CI enforces it.

### Tests

Vitest. Unit tests mock gRPC transport; integration tests run against a real server (tagged
`@integration`).

---

## UI (`decree-ui`)

Repo: [decree-ui](https://github.com/opendecree/decree-ui)

### Stack

Vite + React + Tailwind CSS. No component library. Build your own accessible components from
HTML elements and Tailwind utilities.

### Runtime Dependencies

Current runtime deps: `react`, `react-dom`, `react-router-dom`, `@tanstack/react-query`,
`openapi-fetch`. Keep this list short. A new UI component does not justify a new package.

### Accessibility

Use semantic HTML elements (`<button>`, `<nav>`, `<main>`, `<section>`) over `<div>` where
meaning exists. Add ARIA attributes when the semantic element alone is insufficient. All
interactive elements must be keyboard-reachable.

### TypeScript

Vite bundler mode with strict checks enabled. `exactOptionalPropertyTypes` is not set (bundler
mode difference from the SDK), but `noUnusedLocals`, `noUnusedParameters`, and
`noFallthroughCasesInSwitch` are enforced.

### Lint and Format

Biome. Run `npx biome check --write src/ test/` before committing.

### Tests

Vitest + React Testing Library for unit/component tests. Playwright for E2E against the Docker
Compose stack.
