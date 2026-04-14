# Admin GUI — Design Context

Repo: `opendecree/decree-ui`

## Stack

| Concern | Tool | Why |
|---------|------|-----|
| Framework | React 19 | Standard, largest ecosystem |
| Build | Vite | Standard for React, fast |
| Routing | React Router 7 | Standard, most adopted |
| Data fetching | TanStack Query | De facto standard for async state |
| API client | openapi-typescript + openapi-fetch | Generated from OpenAPI spec |
| HTTP transport | REST gateway | Standard fetch, no gRPC-web needed |
| Styling | Tailwind CSS 4 | Utility-first, no component library lock-in |
| Components | Hand-written | Vanilla — build what you need |
| Testing | Vitest + Testing Library | Standard |
| Lint | Biome | Consistent with TS SDK |

**NOT in stack:** No admin framework (Refine, React Admin), no component library (shadcn, MUI), no form library, no gRPC-web, no WebSocket in v0.1.0.

## Communication

```
decree-ui (browser) → fetch → decree-server:8080/v1/* (REST gateway)
```

## Layout Modes

| Mode | Navigation | Use case |
|------|-----------|----------|
| **full** (default) | Schemas → Tenants → Config → Audit | SaaS platform admin |
| **single-schema** | Tenants → Config → Audit | One product, many tenants |
| **single-tenant** | Config editor + Audit | Simplest — one app, one tenant |

## Two-Phase Config Editing

Phase 1 (Edit): local dirty state in `Map<fieldPath, string>`, visual indicators, per-field undo, constraint validation.

Phase 2 (Submit): review panel shows all changes (old → new), optional description, atomic `SetFields` call, clear on success, show errors on failure.

## Type-Aware Inputs

| Field type | Input widget | Constraint hints |
|-----------|-------------|-----------------|
| string | Text input | minLength/maxLength, pattern |
| integer | Number input (step=1) | min/max, enum as dropdown |
| number | Number input | min/max |
| bool | Toggle switch | — |
| duration | Text input with format hint | min/max |
| timestamp | Date/time picker | — |
| url | URL input with validation | — |
| json | Textarea | JSON Schema validation |
| enum | Dropdown select | Populated from constraint |

## Deployment Modes

1. **Standalone Docker image** — Nginx serves SPA, proxies `/v1/*`
2. **Embedded in decree server** — `embed.FS` at `/admin/`, `ENABLE_UI` env var
3. **Embeddable via iframe** — `/embed` route strips chrome, URL params for context

## Key Decisions

1. Vanilla React — no admin framework, full control
2. REST gateway — standard fetch, no gRPC-web
3. openapi-typescript — generated API client, specs-first
4. Two-phase editing — local dirty state → review → atomic submit
5. Layout modes — same components, config controls depth
6. Dev-mode auth — header bar, no login page in v0.1.0
7. Desktop-first — admin panel, not mobile
