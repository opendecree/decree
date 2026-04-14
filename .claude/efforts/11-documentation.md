# Documentation

**Status:** Phases 1+2 Complete (hand-written content remaining)
**Started:** 2026-03-27

---

## Goal

Comprehensive documentation for the Central Config Service — server, CLI, SDKs, and concepts. Maximize generated docs to minimize maintenance burden. Store everything under `docs/`, committed to git, rendered via MkDocs Material.

---

## Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | MkDocs now or plain markdown? | **MkDocs Material now**, Docker for tooling (no local Python) |
| 2 | SDK doc generator? | **pkg.go.dev** — modules are public, just link to it |
| 3 | Make targets? | **Separate + umbrella**: `make docs`, `docs-api`, `docs-cli`, `docs-serve`, `docs-deploy` |
| 4 | Commit or generate on deploy? | **Commit generated docs**, CI only verifies `make docs` produces no diff |
| 5 | Proto doc structure? | **One file per proto file** via custom template (schema-service, config-service, audit-service, types) |

---

## Final Stack

| Layer | Tool | Output | Notes |
|-------|------|--------|-------|
| Proto API reference | **protoc-gen-doc** (custom template) | Markdown → `docs/api/` | One file per proto file |
| CLI reference | **cobra/doc** (`GenMarkdownTree`) | Markdown → `docs/cli/` | One file per command |
| SDK reference | **pkg.go.dev** | Hosted | Links from doc site, zero generation |
| Concepts, tutorials, ops | Hand-written | Markdown | `docs/concepts/`, `docs/server/` |
| Doc site | **MkDocs Material** (Docker) | Static HTML → GitHub Pages | `mkdocs.yml` defines navigation |
| Format | **Pure markdown** | No MDX | Portable, renders on GitHub natively |

### Why this stack:
- All generators output markdown — single format, reviewable in PRs
- MkDocs Material is the vanilla choice — widely adopted, minimal tooling
- Docker for all doc tools — no local Python/pip needed
- Generated docs committed — always browsable on GitHub without build step
- Cross-linking via standard relative markdown links
- SDK docs via pkg.go.dev — zero maintenance, the Go standard

---

## Directory Structure

```
docs/
├── concepts/                  # Hand-written
│   ├── overview.md
│   ├── schemas-and-fields.md
│   ├── tenants.md
│   ├── typed-values.md
│   ├── versioning.md
│   ├── auth.md
│   └── subscriptions.md
├── api/                       # Generated: protoc-gen-doc
│   ├── schema-service.md
│   ├── config-service.md
│   ├── audit-service.md
│   └── types.md
├── cli/                       # Generated: cobra/doc
│   ├── ccs.md
│   ├── ccs_schema.md
│   ├── ccs_schema_create.md
│   ├── ccs_config.md
│   └── ...
├── server/                    # Hand-written
│   ├── configuration.md
│   ├── deployment.md
│   └── observability.md
├── getting-started.md         # Hand-written tutorial
└── sdk.md                     # Links to pkg.go.dev per module
mkdocs.yml                     # MkDocs config + navigation
```

---

## Makefile Targets

```makefile
make docs          # runs docs-api + docs-cli
make docs-api      # protoc-gen-doc → docs/api/
make docs-cli      # cobra/doc → docs/cli/
make docs-serve    # local MkDocs preview (Docker)
make docs-deploy   # push to GitHub Pages (Docker)
```

---

## Implementation Plan

### Phase 1: Generation infrastructure (completed)
- [x] Add protoc-gen-doc to Dockerfile.tools
- [x] Add protoc-gen-doc via buf.gen.doc.yaml — single API reference file (1529 lines)
- [x] Create `ccs gen-docs` hidden subcommand — cobra/doc generates 37 CLI markdown files
- [x] Add `make docs`, `make docs-api`, `make docs-cli` targets
- [x] Set up `docs/` directory structure
- [x] Generate and commit initial docs

### Phase 2: MkDocs site (completed)
- [x] Using official `squidfunk/mkdocs-material` Docker image
- [x] `mkdocs.yml` with full navigation for all sections
- [x] `make docs-serve` — local preview at localhost:8000
- [x] `make docs-deploy` — push to GitHub Pages
- [x] `docs/sdk.md` with links to pkg.go.dev

### Phase 3: Hand-written content
- [ ] Concepts: overview, schemas & fields, tenants, typed values, versioning, auth, subscriptions
- [ ] Getting started tutorial (end-to-end walkthrough)
- [ ] Server: configuration reference, deployment guide, observability setup

### Phase 4: CI
- [ ] CI step: run `make docs`, verify no diff (generated docs are up-to-date)

---

## Discovery Findings (reference)

### Proto Doc Generators Evaluated

| Tool | Output | Verdict |
|------|--------|---------|
| **protoc-gen-doc** | Markdown, HTML, JSON, custom templates | **Selected** — integrates into buf, Docker, preserves comments |
| Buf BSR | Web UI | Rejected — requires hosted BSR |
| protoc-gen-openapiv2 | OpenAPI/Swagger | Rejected — wrong paradigm for pure gRPC |
| Sabledocs | HTML | Rejected — Python dep, 2-step workflow |

### Go Doc Generators Evaluated

| Tool | Output | Verdict |
|------|--------|---------|
| **pkg.go.dev** | Hosted HTML | **Selected** — automatic for public modules, zero effort |
| gomarkdoc | Markdown | Not needed — pkg.go.dev covers it |
| doc2go | HTML | Not needed — pkg.go.dev covers it |
| pkgsite | HTML (server only) | Not needed — pkg.go.dev covers it |

### Doc Site Frameworks Evaluated

| Tool | Verdict |
|------|---------|
| **MkDocs Material** | **Selected** — vanilla, widely adopted, markdown-native, Docker support |
| Hugo + Docsy | Rejected — complex config, steep learning curve |
| Docusaurus | Rejected — Node.js dependency, overkill |
| Plain markdown | Rejected as primary — generated docs need navigation/search |

### Real-World Project Patterns

- pkg.go.dev is universal for Go SDK reference
- Proto API docs are rare — most projects treat .proto files as docs
- Hugo + Docsy is the CNCF standard, MkDocs is simpler for our scale
- Hand-written always needed for concepts, tutorials, operations
- etcd uses protodoc (custom) for proto docs; gRPC-Go uses Hugo + Docsy
