# ADR-007: Strip Inter-Module `replace` Directives at Release Time

**Date:** 2026-06-29
**Status:** Accepted
**Refines:** [ADR-006](ADR-006-go-module-split.md) — implements its "published versions use normal module paths" intent; does not change the module split or local-dev model.
**Deciders:** OpenDecree maintainers

## Context

ADR-006 split the repo into multiple Go modules and chose `replace` directives in
each module's `go.mod` (pointing at the local worktree) as the local-development
resolution mechanism. It stated that *"published versions use normal module paths and
semantic version tags"* — but nothing enforced that, and the `replace` directives were
present in every tagged commit.

Verifying the documented quickstart from a clean machine (issue #990) showed the
consequence: **the headline install paths are impossible for end users.**

- `go install` refuses any module whose `go.mod` contains `replace` directives:

  > The go.mod file for the module providing named packages contains one or more
  > replace directives.

  Both `go install .../cmd/server@<ver>` and `go install .../cmd/decree@<ver>` fail for
  **every** published version (v0.5.0 … v0.12.0-alpha.4), confirmed in a clean
  `golang:1.25` container.

- The same directives, combined with all versions being `-alpha` pre-releases
  (`@latest` skips pre-releases), made `go get .../sdk/<m>@latest` resolve to stale
  releases.

### Why not commit `go.work` instead?

The obvious alternative — drop `replace`, commit a `go.work` for resolution — was
prototyped and rejected. `replace` resolves locally while preserving each module's
**own** dependency versions; `go.work` recomputes a single **union** MVS across all
26 modules, upgrading shared deps for the lightweight SDK modules. That changes what
every module builds/tests against (no longer what consumers actually get) and, in
practice, surfaced a flaky deadlock in `configwatcher`'s race tests (~20–40% of runs
under workspace mode, 0% single-module). Committing `go.work` would push that flake
into CI. So local development keeps `replace`; the published artifact is fixed instead.

## Decision

Keep `replace` directives in the working tree (`main` is unchanged — CI and local dev
behave exactly as before). **Strip them only from the tagged release commit**, so
published modules carry clean `go.mod` files with normal `require` directives.

`scripts/release/strip-replaces.sh` provides the mechanism:

- `strip` — remove internal `replace github.com/opendecree/decree/...` directives from
  the **published** modules (everything except `examples/`, `e2e`, `chaos`, `stress`,
  `fixtures/`, which keep theirs).
- `tidy` — `GOWORK=off go mod tidy` per module, leaf-first, so each module's `go.sum`
  gains the internal-module hashes the published tag needs. (Under `replace`, those
  hashes are absent; without them `go install` fails with *"missing go.sum entry"* —
  the concrete second failure found while validating the fix.)
- `check` — fail if any published module still has an internal `replace`.

## Release procedure (extends RELEASING.md)

The existing flow already merges a PR bumping every intra-repo `require` to the new
version before tagging. The strip step is added at tag time, on the release commit
(not merged back to `main`):

1. Merge the require-version-bump PR to `main` (unchanged).
2. On the release commit: `strip-replaces.sh strip`.
3. Tag **leaf-first** (`api`, `sdk/retry`, … `cmd/decree`, root). Before tagging each
   module, run `strip-replaces.sh tidy <module>` — its intra-repo deps are already
   published from earlier iterations, so `go mod tidy` resolves and records their
   hashes. Push tags individually, ≤3 at a time (GitHub's tag-event limit).
4. `strip-replaces.sh check` must pass on the tagged tree.
5. Install commands must pin an explicit version (e.g. `@v0.12.0-alpha.5`) until a
   non-pre-release exists — `@latest` skips pre-releases. Release tagging must also
   flag pre-releases consistently so GitHub's "Latest" release is correct.

## Consequences

**Positive:**
- `go install github.com/opendecree/decree/cmd/{server,decree}@<version>` works.
- `go get github.com/opendecree/decree/sdk/<module>@<version>` works.
- `main`, CI, and the local-dev workflow are untouched — zero new flakiness, no
  `go.work` policy change, no churn on every `go.mod`.

**Negative / notes:**
- The tagged commit diverges from `main` (it has no `replace` directives). It is a
  release artifact, not merged back.
- The release runbook gains a strip + per-module tidy step; it must run leaf-first.
- Takes effect only from the first release tagged this way; tags ≤ v0.12.0-alpha.4
  remain non-installable.
