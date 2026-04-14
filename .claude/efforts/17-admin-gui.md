# Admin GUI

**Status:** In Progress (Phases 1-4 complete, Phase 5 next)
**Started:** 2026-04-09
**Last Updated:** 2026-04-13
**Repo:** `opendecree/decree-ui`

---

## Goal

A web-based admin interface for OpenDecree. Pluggable (embeddable in existing admin panels via iframe), standalone-capable, supports multiple layout modes (single/multi schema, tenant, config). Two-phase config editing with atomic bulk submit.

## Stack

| Concern | Tool | Why |
|---------|------|-----|
| Framework | React 19 | Standard, largest ecosystem |
| Build | Vite | Standard for React, fast |
| Routing | React Router 7 | Standard, most adopted |
| Data fetching | TanStack Query | De facto standard for async state |
| API client | openapi-typescript + openapi-fetch | Generated from OpenAPI spec, zero hand-written fetch code |
| HTTP transport | REST gateway | Standard fetch, no gRPC-web needed |
| Styling | Tailwind CSS 4 | Utility-first, no component library lock-in |
| Components | Hand-written | Vanilla — build what you need |
| Testing | Vitest + Testing Library | Standard |
| Lint | Biome | Consistent with TS SDK |

**Runtime deps: React, React Router, TanStack Query, Tailwind.** API client is generated at build time.

### NOT in stack

- No admin framework (Refine, React Admin) — vanilla approach
- No component library (shadcn, MUI) — hand-written components
- No form library — `useState` with `Map<fieldPath, string>` for edit state
- No code editor in v0.1.0 — plain textarea for YAML/JSON, CodeMirror later
- No gRPC-web — REST gateway already exists
- No WebSocket — polling via TanStack Query for v0.1.0

## Communication

The GUI talks to the REST gateway (`HTTP_PORT`). Standard `fetch()` calls. The API client is auto-generated from the OpenAPI spec using `openapi-typescript`, ensuring types always match the server.

```
decree-ui (browser) → fetch → decree-server:8080/v1/* (REST gateway)
```

## Authentication

### Dev mode (v0.1.0)
Header bar with text input for `x-subject` + role dropdown. No login page. Headers attached to every request.

### JWT mode (later)
Redirect to IdP, store token in memory, attach as Bearer. Auto-detect based on server config.

## Layout Modes

Same components, different navigation depth. Controlled by config:

| Mode | Navigation | Use case |
|------|-----------|----------|
| **full** (default) | Schemas → Tenants → Config → Audit | SaaS platform admin |
| **single-schema** | Tenants → Config → Audit | One product, many tenants |
| **single-tenant** | Config editor + Audit | Simplest — one app, one tenant |

URL structure:
```
/schemas                          # Full: schema list
/schemas/:id                      # Schema detail + versions
/schemas/:id/tenants              # Tenant list for schema
/tenants/:id/config               # Config editor (all modes)
/tenants/:id/config/versions      # Version history + rollback
/tenants/:id/audit                # Audit log
```

Single-tenant mode skips straight to `/tenants/:id/config`.

## Two-Phase Config Editing

### Phase 1: Edit
- User modifies values in the config editor
- Changes tracked locally in `Map<fieldPath, string>` (dirty state)
- Modified fields get a visual indicator (colored dot, highlight)
- Undo per field (reset to server value)
- Reset all (discard all pending changes)
- Local validation against schema constraints before submit

### Phase 2: Submit
- Review panel shows all pending changes (old → new per field)
- Optional description field (for audit log)
- One `SetFields` call (atomic bulk write)
- On success: clear dirty state, refetch config
- On error: show which fields failed, keep dirty state

```
┌─────────────────────────────────────────┐
│ Config: acme                       [⟳] │
│                                         │
│ payments.fee         [0.5%      ]  •    │
│ payments.retries     [3         ]       │
│ payments.enabled     [■ on      ]  •    │
│ payments.currency    [USD       ]       │
│                                         │
│ ┌─────────────────────────────────────┐ │
│ │ 2 pending changes                   │ │
│ │ payments.fee:     0.3% → 0.5%      │ │
│ │ payments.enabled: off → on          │ │
│ │                                     │ │
│ │ Description: [quarterly adjustment] │ │
│ │              [Cancel]  [Apply]      │ │
│ └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Type-Aware Config Inputs

The config editor renders appropriate widgets based on the schema field type:

| Field type | Input widget | Constraint hints |
|-----------|-------------|-----------------|
| string | Text input | minLength/maxLength, pattern |
| integer | Number input (step=1) | min/max, enum as dropdown |
| number | Number input | min/max, exclusiveMin/Max |
| bool | Toggle switch | — |
| duration | Text input with format hint (e.g., "30s", "1h") | min/max |
| timestamp | Date/time picker | — |
| url | URL input with validation | — |
| json | Textarea (CodeMirror later) | JSON Schema validation |
| enum | Dropdown select | Populated from constraint |

Constraints are fetched from the schema and rendered as help text, placeholders, or input attributes.

## Pages

### Schema Management
| Page | RPCs | Description |
|------|------|-------------|
| Schema list | ListSchemas | Table with search/filter |
| Schema detail | GetSchema, ExportSchema | Field table, version list, YAML preview |
| Schema editor | UpdateSchema, PublishSchema | Add/remove/edit fields + constraints |
| Schema import | ImportSchema | YAML paste/upload with preview |

### Tenant Management
| Page | RPCs | Description |
|------|------|-------------|
| Tenant list | ListTenants | Table with schema filter |
| Tenant create | CreateTenant | Name + schema + version form |
| Tenant detail | GetTenant, UpdateTenant, DeleteTenant | Settings |

### Config Editor (core)
| Page | RPCs | Description |
|------|------|-------------|
| Config editor | GetConfig, GetSchema, SetFields | Type-aware inputs, 2-phase edit |
| Field locks | ListFieldLocks, LockField, UnlockField | Toggle per field |
| Config versions | ListVersions, GetVersion | Version table |
| Config rollback | RollbackToVersion | Confirm + rollback |
| Config diff | GetVersion (x2) | Side-by-side comparison |
| Config import/export | ImportConfig, ExportConfig | YAML up/download |

### Audit & Usage
| Page | RPCs | Description |
|------|------|-------------|
| Audit log | QueryWriteLog | Filterable table with pagination |
| Usage stats | GetFieldUsage, GetTenantUsage, GetUnusedFields | Dashboard tables |

## Deployment Modes

### 1. Standalone Docker image (default)
```bash
docker run -e DECREE_API_URL=http://decree:8080 ghcr.io/opendecree/decree-ui
```
Nginx/Caddy serves the SPA, proxies `/v1/*` to the decree server.

### 2. Embedded in decree server binary
```go
//go:embed ui/dist/*
var uiAssets embed.FS
```
Users get the GUI at `http://localhost:8080/admin/` with zero extra deployment. Build artifacts fetched during CI or committed. Controlled by `ENABLE_UI` env var.

### 3. Embeddable via iframe
```html
<iframe src="https://decree-ui/embed?tenant=acme&layout=single-tenant" />
```
The `/embed` route strips outer chrome (no sidebar, no header). URL params control layout mode and pre-selected tenant/schema.

## Project Structure

```
decree-ui/
├── src/
│   ├── main.tsx                  # Entry point
│   ├── App.tsx                   # Router + layout + TanStack Query provider
│   ├── api/                      # Generated API client
│   │   ├── schema.ts             # openapi-fetch typed operations
│   │   └── client.ts             # Base client with auth headers
│   ├── pages/
│   │   ├── schemas/              # Schema list, detail, editor, import
│   │   ├── tenants/              # Tenant list, create, detail
│   │   ├── config/               # Config editor, versions, diff, audit
│   │   └── embed/                # Stripped layout for iframe embedding
│   ├── components/
│   │   ├── Layout.tsx            # Sidebar + header + content
│   │   ├── Table.tsx             # Reusable data table
│   │   ├── TypedInput.tsx        # Type-aware field input
│   │   ├── DiffView.tsx          # Side-by-side config diff
│   │   ├── PendingChanges.tsx    # Review panel for 2-phase edit
│   │   └── AuthBar.tsx           # Dev mode subject/role input
│   └── lib/
│       ├── config.ts             # Layout mode, API URL, env
│       ├── types.ts              # Generated from OpenAPI (openapi-typescript)
│       └── hooks.ts              # Shared TanStack Query hooks
├── public/
├── index.html
├── package.json
├── vite.config.ts
├── tailwind.config.ts
├── biome.json
├── tsconfig.json
└── .github/workflows/
    └── ci.yml
```

## Implementation Phases

### Phase 1: Scaffold + API client + auth bar — COMPLETE (2026-04-12)
- [x] Create repo `opendecree/decree-ui`
- [x] Vite + React + React Router + TanStack Query + Tailwind
- [x] Generate API types from OpenAPI spec (openapi-typescript + swagger2openapi)
- [x] API client with auth header injection (openapi-fetch)
- [x] Dev-mode auth bar (subject/role from constants, persisted)
- [x] Layout shell (sidebar + header + content area)
- [x] CI workflow (lint, typecheck, test, build)
- [x] Dark mode (class-based toggle, system preference default)
- [x] Icons and constants extracted to dedicated modules

### Phase 2: Schema pages — COMPLETE (2026-04-13)
- [x] Schema list with search filter, published/draft badges
- [x] Schema detail: type badges, collapsible fields, path-prefix grouping
- [x] Schema detail: description column, deprecated badge, field count, info block
- [x] Schema import (YAML textarea + auto-publish)
- [x] Schema export (YAML download)

### Phase 3: Tenant pages — COMPLETE (2026-04-13)
- [x] Tenant list with name filter, schema info
- [x] Tenant create form with schema selector
- [x] Tenant detail + delete with confirmation

### Phase 4: Config editor (core) — COMPLETE (2026-04-13)
- [x] Config editor merged into tenant detail (read-only by default, edit mode toggle)
- [x] Type-aware inputs: string, int, number, bool toggle, duration, time, url, json, enum dropdown
- [x] Two-phase edit: dirty tracking + pending changes review panel + atomic submit
- [x] Field lock indicators + lock/unlock toggle
- [x] Role-based permissions (hide Edit/Delete/locks per role)
- [x] Customization: config/labels.json + config/theme.json for white-labeling
- [x] Duration display formatting (86400s → 24h)
- [x] Playwright screenshot tooling

### Phase 5: Config history
- [ ] Version list with timestamps
- [ ] Config diff (side-by-side)
- [ ] Rollback with confirmation
- [ ] Config import/export (YAML)

### Phase 6: Audit + usage
- [ ] Audit log table with filters (tenant, field, actor, date range)
- [ ] Pagination
- [ ] Usage stats tables

### Phase 7: Layout modes + embedding
- [ ] Layout mode config (full, single-schema, single-tenant)
- [ ] `/embed` route (stripped chrome)
- [ ] URL params for pre-selected tenant/schema

### Phase 8: Embed in Go binary
- [ ] Build step in decree CI that builds the UI
- [ ] `embed.FS` serving at `/admin/`
- [ ] `ENABLE_UI` env var to enable/disable

### Phase 9: Polish + release
- [ ] Responsive tweaks
- [ ] Loading states, error boundaries
- [ ] README, docs, CONTRIBUTING, governance
- [ ] Docker image build + publish

## Key Decisions

1. **Vanilla React** — no admin framework, full control, fewer deps
2. **REST gateway** — standard fetch, no gRPC-web complexity
3. **openapi-typescript** — generated API client from spec, consistent with specs-first approach
4. **Tailwind** — utility CSS, no component library lock-in
5. **TanStack Query** — de facto standard, handles caching/invalidation/polling
6. **Hand-written components** — no shadcn, no MUI, build what you need
7. **Two-phase editing** — local dirty state → review → atomic bulk submit via SetFields
8. **Layout modes** — same components, config controls navigation depth
9. **Embeddable via iframe** — `/embed` strips chrome, URL params for context
10. **Embeddable in Go binary** — `embed.FS` for zero-deployment GUI
11. **Dev-mode auth** — header bar with subject/role, no login page in v0.1.0
12. **Dark mode** — Tailwind `dark:` classes, system preference
13. **Polling for v0.1.0** — TanStack Query refetch interval, streaming later
14. **Textarea for YAML/JSON** — CodeMirror added later if needed
15. **Desktop-first** — admin panel, not a mobile app
