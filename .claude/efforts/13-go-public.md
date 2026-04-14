# Go Public Checklist

**Status:** In Progress
**Started:** 2026-03-29

---

## Goal

Prepare the repo for public release on GitHub. Ensure it's clean, professional, and ready for community consumption.

## Must-Do (before flipping to public)

- [x] **Scan for secrets** — gitleaks: no leaks found (46 commits scanned)
- [x] **LICENSE** — Apache 2.0 verified
- [x] **README review** — badges, power tools section, architecture diagram fix
- [x] **Module paths** — all 8 modules confirmed under `github.com/opendecree/decree`
- [x] **Remove GOPRIVATE** — not set anywhere, nothing to remove
- [x] **Clean git history** — squashed Docker/CI/effort chains (46→40 commits)
- [x] **GitHub repo settings** — description set, topics added (go, grpc, configuration, multi-tenant, schema-driven)

## Should-Do (before or shortly after)

- [x] **Flip to public** — already public
- [x] **Tag v0.1.0** — tagged all 7 submodules + root; Go proxy indexed
- [x] **Branch protection** — classic protection on main (require PR reviews + status checks)
- [x] **Issue templates** — bug report + feature request
- [x] **SECURITY.md** — vulnerability reporting instructions
- [x] **Code of Conduct** — Contributor Covenant v2.1 (by reference)
- [x] **GitHub Discussions** — enabled
- [x] **Fix CI** — disabled setup-go cache, fixed stale pseudo-versions → tagged v0.1.0 submodules, fixed gofumpt/docs/adminclient type mapping, refreshed go.sum checksums

## Nice-to-Have

- [x] **README badges** — CI status, Go version, license
- [ ] **Docker layer caching** — add cache-from/to in CI Docker build steps
- [ ] **Social preview** — repo card image for link sharing
- [ ] **Example repo** — separate repo with runnable examples
- [ ] **Blog post / announcement** — introduce OpenDecree

## Implementation Order

1. ~~Scan for secrets~~ done
2. ~~License + module path confirmation~~ done
3. ~~Clean git history~~ done
4. ~~README final review~~ done
5. ~~GitHub repo settings~~ done
6. ~~Flip to public~~ already done
7. ~~Tag v0.1.0~~ done
8. ~~Fix CI~~ done (Build + Test + Lint pass; Docs + E2E in progress)
9. Verify ghcr.io image push
