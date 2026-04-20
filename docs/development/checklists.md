# Checklists

Standard development workflow checklists.

---

## Before Commit

- [ ] `go build ./...` compiles clean (root module)
- [ ] `go vet ./...` passes
- [ ] `gofumpt -l .` reports no files (formatting)
- [ ] All modified SDK modules build: `cd sdk/<mod> && go build ./...`
- [ ] All tests pass: `go test ./internal/...` + SDK modules + `cmd/decree`
- [ ] Coverage ratchet passes: `./scripts/check-coverage.sh`
- [ ] No secrets, credentials, or tokens in staged files
- [ ] No binary files accidentally staged
- [ ] Commit message follows convention (imperative, explains why)
- [ ] Co-Authored-By line included

## Before PR

- [ ] All "Before Commit" checks pass
- [ ] Branch is up to date with main (`git rebase origin/main`)
- [ ] New/changed env vars documented in `docs/server/configuration.md`
- [ ] New CLI commands have `Short` and `Long` descriptions
- [ ] Generated docs are up to date: `make docs` produces no diff
- [ ] OpenAPI spec in sync: `cmd/server/openapi.json` matches `docs/api/openapi.swagger.json`
- [ ] Coverage didn't drop — if it did, add tests or adjust threshold with justification
- [ ] Update coverage badge if changed: `./scripts/coverage.sh` (server), update README badge
- [ ] Agent context updated if relevant (`.agents/context/`)
- [ ] PR description includes Summary, Test plan
- [ ] No TODO/FIXME introduced without a corresponding GitHub issue

## After PR Merge

- [ ] Switch to main: `git checkout main && git pull --rebase`
- [ ] Delete local branch: `git branch -d <branch>`
- [ ] Verify CI passed on main (check GitHub Actions)
- [ ] Update agent context if task completes a milestone item

## Before Release Tag

- [ ] All CI checks pass on main (lint, test, build, docs, e2e)
- [ ] All milestone issues closed or deferred to next milestone
- [ ] Agent context updated: design summaries in `.agents/context/completed.md`
- [ ] `go.mod` versions consistent across all modules
- [ ] README accurate: features, install commands, env vars, architecture
- [ ] CONTRIBUTING accurate: project structure, module layout
- [ ] No "CCS" or stale naming references in docs
- [ ] No merge conflict markers in any file: `grep -r '<<<<<<' .`
- [ ] Gitleaks clean: `docker run --rm -v $(pwd):/path zricethezav/gitleaks:latest git /path`
- [ ] Docker images build and run: `STORAGE_BACKEND=memory` smoke test
- [ ] Coverage ratchet passes
- [ ] Coverage badge is accurate: `./scripts/coverage.sh` matches README badge
- [ ] Go SDK coverage up to date: `go test -cover ./sdk/...` matches README badge
- [ ] Changelog/highlights drafted for release notes

## Release Tag Process

1. Tag root + all submodules:
   ```
   git tag -a v{X.Y.Z} -m "v{X.Y.Z}"
   git tag -a api/v{X.Y.Z} -m "api v{X.Y.Z}"
   git tag -a sdk/configclient/v{X.Y.Z} -m "sdk/configclient v{X.Y.Z}"
   git tag -a sdk/adminclient/v{X.Y.Z} -m "sdk/adminclient v{X.Y.Z}"
   git tag -a sdk/configwatcher/v{X.Y.Z} -m "sdk/configwatcher v{X.Y.Z}"
   git tag -a sdk/grpctransport/v{X.Y.Z} -m "sdk/grpctransport v{X.Y.Z}"
   git tag -a sdk/tools/v{X.Y.Z} -m "sdk/tools v{X.Y.Z}"
   git tag -a cmd/decree/v{X.Y.Z} -m "cmd/decree v{X.Y.Z}"
   ```
2. Push tags: `git push origin --tags`
3. If release workflow doesn't trigger: `gh workflow run release.yml --ref v{X.Y.Z}`
4. Monitor: `gh run list --workflow=release.yml --limit 1`

## Post-Release Verification

- [ ] GitHub Release exists: `gh release view v{X.Y.Z}`
- [ ] Release notes are clean and accurate (no leaked output)
- [ ] Docker images pull: `docker pull ghcr.io/opendecree/decree:{X.Y.Z}`
- [ ] Docker image runs: `docker run --rm -e STORAGE_BACKEND=memory ghcr.io/opendecree/decree:{X.Y.Z}`
- [ ] CLI image pulls: `docker pull ghcr.io/opendecree/decree-cli:{X.Y.Z}`
- [ ] Goreleaser binaries attached (checksums.txt + platform tarballs)
- [ ] BSR module updated: check buf.build/opendecree/decree
- [ ] `go install github.com/opendecree/decree/cmd/decree@v{X.Y.Z}` works
- [ ] pkg.go.dev shows new version (may take a few minutes)
- [ ] Version endpoint reports correct version (once Docker version fix is in)
- [ ] Edit release notes if needed: `gh release edit v{X.Y.Z} --notes "..."`
- [ ] Milestone auto-closed by CI (verify)

## Milestone Lifecycle

Milestones represent efforts (e.g. "Admin GUI", "Security Review"), not releases.

When starting a new effort:
1. Create milestone on GitHub with effort name and description
2. Create issues for each work item and assign to milestone
3. If the work has significant design context, create a doc in `.agents/context/`

When completing an effort:
1. Verify all issues are closed
2. Move design context summaries to `.agents/context/completed.md`
3. Close the milestone
