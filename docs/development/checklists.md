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
- [ ] Coverage didn't drop — check Codecov PR comment for patch/project numbers
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
- [ ] Supply-chain pins valid: `./scripts/check-supply-chain-pins.sh`
- [ ] Docker images build and run: `INSECURE_LISTEN=1 STORAGE_BACKEND=memory` smoke test
- [ ] Coverage ratchet passes
- [ ] Coverage ratchet passes: `./scripts/check-coverage.sh`
- [ ] Codecov badge in README reflects current state (auto-updated by Codecov)
- [ ] Changelog/highlights drafted for release notes

## Release Tag Process

Published modules carry local-dev `replace` directives on `main`, which make
`go install`/`go get` impossible. Per
[ADR-007](../adr/ADR-007-strip-replaces-at-release.md) they are stripped from the
tagged commit only — `main` is never changed. Follow ADR-007's "Release
procedure" exactly; the outline:

1. Merge the require-version-bump PR (every intra-repo `require` → new version;
   README/docs install commands pinned to the new version). `main` keeps its
   `replace` directives.
2. On the release commit: `./scripts/release/strip-replaces.sh strip`.
3. Tag **leaf-first**, in the order below, tidying each module
   (`strip-replaces.sh tidy <module>`) and pushing it before the next so its
   intra-repo dependencies resolve from the module proxy. Push **≤3 tags per
   `git push`** (GitHub drops tag events past 3 in one push). Tag the root
   `v{X.Y.Z}` **last** — only it matches the `v*` trigger and creates the GitHub
   release.
   ```
   api → sdk/retry → sdk/configclient → sdk/configwatcher → sdk/adminclient →
   sdk/grpctransport → sdk/tools → sdk/contrib/envconfig → sdk/contrib/koanf →
   sdk/contrib/viper → contrib/decree-docs → cmd/decree → (root) v{X.Y.Z}
   ```
4. `./scripts/release/strip-replaces.sh check` must pass on the tagged tree.
5. If the release workflow doesn't trigger: `gh workflow run release.yml --ref v{X.Y.Z}`
6. Monitor: `gh run list --workflow=release.yml --limit 1`

> The tagged commit intentionally diverges from `main` (no `replace` directives)
> and is **not** merged back. During alpha, install commands must pin an explicit
> version (`@v{X.Y.Z}`) — `@latest` skips pre-releases.

## Post-Release Verification

- [ ] GitHub Release exists: `gh release view v{X.Y.Z}`
- [ ] Release notes are clean and accurate (no leaked output)
- [ ] Docker images pull: `docker pull ghcr.io/opendecree/decree:{X.Y.Z}`
- [ ] Docker image runs: `docker run --rm -e STORAGE_BACKEND=memory ghcr.io/opendecree/decree:{X.Y.Z}`
- [ ] CLI image pulls: `docker pull ghcr.io/opendecree/decree-cli:{X.Y.Z}`
- [ ] Artifact attestations verified: `gh attestation verify oci://ghcr.io/opendecree/decree:{X.Y.Z} --owner opendecree` (see [SECURITY.md](../../SECURITY.md) for full instructions)
- [ ] Goreleaser binaries attached (checksums.txt + platform tarballs)
- [ ] BSR module updated: check buf.build/opendecree/decree
- [ ] `go install github.com/opendecree/decree/cmd/decree@v{X.Y.Z}` works
- [ ] pkg.go.dev shows new version (may take a few minutes)
- [ ] Version endpoint reports correct version (once Docker version fix is in)
- [ ] Edit release notes if needed: `gh release edit v{X.Y.Z} --notes "..."`
- [ ] Milestone auto-closed by CI (verify)

## Supply-chain pinning policy

To prevent silent base-image or third-party-action substitution, the repo
enforces these rules. CI runs `scripts/check-supply-chain-pins.sh` to keep
PRs honest.

- **Base images.** Every `FROM` line in `build/*` must pin to an
  `image@sha256:<digest>`. Add a comment on the line above recording the
  human-readable tag (e.g. `# golang:1.26-bookworm`). Dependabot's
  `docker` ecosystem keeps these digests current.
- **Third-party actions.** Every `uses:` reference to an action outside a
  trusted org must pin to a 40-char commit SHA, with a comment on the line
  above recording the original tag. Trusted orgs (`actions/`, `github/`,
  `docker/`) may continue to pin by major-version tag. Dependabot's
  `github-actions` ecosystem updates SHA pins automatically when the
  original ref is itself a SHA.
- **`docker run` invocations** in workflow steps follow the same digest
  rule as `FROM` lines.

When adding a new external action, run
`gh api repos/<owner>/<repo>/git/ref/tags/<tag> --jq '.object.sha'` to
resolve a tag to its commit SHA before pinning.

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
