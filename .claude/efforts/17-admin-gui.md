# Admin GUI

**Status:** Planning
**Started:** 2026-04-09
**Repo:** `opendecree/decree-ui` (separate)

---

## Goal

Web UI for visual schema/config management. Alpha quality. Pluggable — users can replace or disable it.

## Stack

React + TypeScript + shadcn/ui. Talks to REST gateway (standard fetch).

## Architecture

- SPA served at `/ui/` path on the server
- Built static files embedded in server binary via Go `embed.FS`
- `ENABLE_UI` env var (default: true)
- `UI_PATH` env var overrides embedded files with local directory
- Can be hosted separately (point at any decree server)

## Alpha Scope (essential ops)

- [ ] **Tenant switcher** — dropdown, one tenant visible at a time, user may have access to multiple
- [ ] **Schema browser** — view fields, types, constraints, versions
- [ ] **Config editor** — view/edit field values, types shown inline
- [ ] **Tenant list** — create, view, switch between tenants

### NOT in alpha
- Audit logs viewer
- Field locks management
- Version history / diff
- Import/export
- User/role management

## Server-Side Integration (main repo)

- [ ] `internal/server/ui.go` — embed.FS handler for static files
- [ ] `ENABLE_UI` + `UI_PATH` env vars
- [ ] Serve at `/ui/` prefix, SPA fallback for client-side routing

## Key Decisions
- Separate repo for independent development and build
- shadcn/ui — copy-paste components, no framework lock-in
- REST gateway is the only backend — no custom API for the GUI
- Alpha = functional but not polished
