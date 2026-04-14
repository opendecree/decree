# Release Improvements

**Status:** Planning
**Started:** 2026-04-09

---

## Goal

Fix issues discovered during v0.3.1 release and optimize the release pipeline.

## Issues Found

### Docker image shows "dev" version
The server binary inside Docker images reports `version: "dev"` and `commit: "unknown"` because the Dockerfile doesn't inject ldflags during build. The Goreleaser-built binaries have correct versions, but Docker images don't.

**Root cause:** `build/Dockerfile` runs `go build ./cmd/server` without `-ldflags` for version injection.

**Fix:** Pass build args for version/commit in the Dockerfile and set them from CI:

```dockerfile
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -ldflags "-s -w \
    -X github.com/opendecree/decree/internal/version.Version=${VERSION} \
    -X github.com/opendecree/decree/internal/version.Commit=${COMMIT}" \
    -o /bin/decree ./cmd/server
```

CI passes `--build-arg VERSION=${GITHUB_REF_NAME#v} --build-arg COMMIT=${GITHUB_SHA::7}`.

Same fix needed for `build/Dockerfile.decree` (CLI image).

### Tag push doesn't trigger release workflow
GitHub Actions didn't fire on tag push when tags were deleted and re-created. Had to add `workflow_dispatch` as a fallback and trigger manually.

**Status:** Workaround in place (workflow_dispatch added). Not a code fix — GitHub Actions behavior.

### Release notes had docker pull output leaked
The release notes generation step captured docker pull output in the Install section.

**Root cause:** The `GITHUB_REF_NAME` variable expansion in the release body script wasn't properly escaped, and docker pull output from a previous step leaked into the output.

**Fix:** Review the release notes generation script and ensure shell variable expansion is correct. Consider using a static install template instead of dynamic generation.

### apidiff requires Go 1.25
The `apidiff` tool requires Go 1.25 but CI pins Go 1.24 with `GOTOOLCHAIN=local`.

**Status:** Workaround in place (`GOTOOLCHAIN=auto` + `continue-on-error`). Will resolve naturally when project upgrades to Go 1.25.

## Optimization Ideas

### Parallelize release jobs
Currently: validate → release notes → goreleaser (sequential).
Images and BSR already run in parallel with release notes.

Goreleaser depends on release notes because it uses `mode: append` (appends binaries to the release created by the notes job). Could switch goreleaser to `mode: replace` and have it create the release directly, removing the dependency on release notes.

### Cache Go modules in goreleaser
Goreleaser downloads all Go modules from scratch. Adding `actions/setup-go` with caching before the goreleaser step would speed up builds.

### Multi-platform Docker images
Currently single-platform (linux/amd64). Could add `platforms: linux/amd64,linux/arm64` to docker/build-push-action for ARM support (e.g., AWS Graviton, Apple Silicon).

## Work Items

- [x] **Fix Docker version injection** — ARG VERSION/COMMIT in both Dockerfiles + CI build args
- [x] **Fix release notes template** — rewritten with heredoc, no shell leakage
- [x] **Go module caching in goreleaser** — already handled by actions/setup-go@v6
- [x] **Multi-platform Docker images** — linux/amd64 + linux/arm64 in both main.yml and release.yml
- [ ] **Post-release checklist** — automated verification script
