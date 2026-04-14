# Go Public Checklist

**Status:** Near complete (Admin GUI + examples remaining)
**Started:** 2026-03-29
**Updated:** 2026-04-13

---

## Completed

Secret scan, LICENSE, README review, module paths, git history, GitHub settings, repo flipped public, v0.1.0 tagged, branch protection, issue templates, SECURITY.md, Code of Conduct, Discussions, CI fixed. See `completed.md`.

## Pre-Launch (before announcing)

- [x] **REST/HTTP Gateway** (effort 16) — REST/JSON API for all gRPC services
- [x] **Schema enrichment** (effort 20) — OAS-inspired metadata
- [x] **BSR proto publishing** — buf.build/opendecree/decree
- [x] **In-memory storage** — `STORAGE_BACKEND=memory` for zero-dep evaluation
- [x] **GitHub Project** — roadmap board with issues from efforts
- [ ] **Admin GUI** (effort 17) — web UI for config/schema management (alpha)
- [x] **TypeScript SDK** (effort 23) — @opendecree/sdk v0.1.0 on npm, 139 tests/98% cov
- [x] **Python SDK** (effort 18) — code complete, 150 tests/98% cov, awaiting PyPI publish
- [ ] **Examples repo** (effort 19) — runnable examples per language
- [ ] **Final README update** — add GUI/SDK sections when ready
- [x] **Verify ghcr.io image push** — confirmed: :main images pull and run correctly

## Order

1. REST Gateway (effort 16)
2. BSR proto publishing
3. Admin GUI (effort 17) + TS SDK + Python SDK (parallel)
4. Examples repo (effort 19)
5. Final README + announce
