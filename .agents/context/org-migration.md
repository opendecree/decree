# Org Migration: zeevdr/* → opendecree/*

## Overview

Migrate all OpenDecree repos from personal GitHub account (zeevdr) to a new
GitHub organization (opendecree). Clean history, fresh tags, new module paths.

## Audit results: 331 references across 4 repos

### decree repo (291 references)

| Category | Count | Files |
|----------|-------|-------|
| Go module declarations | ~70 | 17 go.mod (8 core + 9 examples) |
| Go import statements | ~120 | 90+ .go source files |
| Proto generated code | ~10 | 5 .pb.go files in api/centralconfig/v1/ |
| Build system (ldflags) | 6 | Makefile:21, .goreleaser.yaml:11-12, build/Dockerfile:16 |
| Proto codegen config | 1 | buf.gen.yaml:6 (go_package_prefix) |
| CI workflows | 7 | release.yml:119-127, project.yml:20 |
| Helm chart | 5 | Chart.yaml:7-12, values.yaml:6 |
| Documentation | ~50 | README.md, docs/*.md, examples/*/README.md |
| Governance + email | 2 | CODE_OF_CONDUCT.md:7, SECURITY.md:9 |
| mkdocs config | 2 | mkdocs.yml:3-4 |
| JSON schema | 1 | schemas/schema-yaml.json:3 |
| Agent context | 4 | .agents/context/completed.md, admin-gui.md |
| Claude skills | 5 | .claude/skills/issue/SKILL.md, issues/SKILL.md |
| go.sum files | ~17 | (regenerated, not manually edited) |

### decree-python (25 references)

| File | Lines |
|------|-------|
| sdk/pyproject.toml | 42-45 (4 URLs) |
| README.md | 3, 6, 7, 9, 81 |
| sdk/README.md | 3, 6, 7, 9, 69, 78 |
| examples/README.md | 63 |
| sdk/docs/quickstart.md | 109 |
| CONTRIBUTING.md | 17 |
| CODE_OF_CONDUCT.md | 7 (email) |
| SECURITY.md | 9 (email) |
| CHANGELOG.md | 22 |
| .github/workflows/project.yml | 20 |

### decree-typescript (11 references)

| File | Lines |
|------|-------|
| package.json | 41, 44, 47 |
| README.md | 3, 6, 7, 9 |
| examples/README.md | 56 |
| CONTRIBUTING.md | 15 |
| CODE_OF_CONDUCT.md | 7 (email) |
| SECURITY.md | 9 (email) |
| CHANGELOG.md | 25 |
| .github/workflows/project.yml | 20 |

### decree-ui (4 references)

| File | Lines |
|------|-------|
| README.md | 3, 4, 6 |
| .github/workflows/project.yml | 20 |

## External services inventory

| Service | Current config | Auth | Migration action |
|---------|---------------|------|-----------------|
| **ghcr.io** | Dynamic via `github.repository_owner` | GITHUB_TOKEN (auto) | Auto-resolves to opendecree |
| **GitHub Releases** | goreleaser → GitHub API | GITHUB_TOKEN (auto) | Auto |
| **GitHub Pages** | mkdocs gh-deploy on push to main | GITHUB_TOKEN (auto) | Enable Pages on new repo |
| **GitHub Projects** | `users/zeevdr/projects/2` | PROJECT_TOKEN (custom PAT) | Create org project, update URL in 4 project.yml files, new PAT |
| **buf.build (BSR)** | `buf.build/opendecree/decree` | BUF_TOKEN (custom) | Verify accepts pushes from new org, recreate token if scoped |
| **PyPI** | `opendecree` package, OIDC trusted publisher | OIDC (id-token) | Update trusted publisher: repo `opendecree/decree-python` → `opendecree/decree-python` |
| **npmjs** | `@opendecree/sdk`, OIDC trusted publisher | OIDC (id-token) | Update trusted publisher: repo `opendecree/decree-typescript` → `opendecree/decree-typescript` |
| **Go module proxy** | Indexes from git tags | Public | Auto — new tags on new repo |
| **pkg.go.dev** | Indexes from module proxy | Public | Auto — appears within ~30 min of tag |

### Secrets to recreate in org

| Secret | Scope | Used by |
|--------|-------|---------|
| `BUF_TOKEN` | org-level | decree release.yml |
| `PROJECT_TOKEN` | org-level | all 4 repos project.yml |
| `GITHUB_TOKEN` | auto per repo | no action needed |

### Environments to create

| Repo | Environment | Purpose |
|------|------------|---------|
| decree-python | `pypi` | OIDC trusted publisher for PyPI |
| decree-typescript | `npm` | OIDC trusted publisher for npmjs |

## Key decisions

- **Org name**: `opendecree`
- **Approach**: Create fresh repos (NOT transfer) with squashed history
- **Version**: Tag v0.3.0 (decree) and v0.2.0 (SDKs, UI) as first org releases
- **Email**: Use GitHub-native channels for now (Issues for conduct, Security Advisories for vulnerabilities). Switch to custom domain emails later when opendecree.dev is set up.

## Execution plan

### Phase 0: Pre-flight (before touching anything)

- [ ] Verify `opendecree` org name is available on GitHub
- [ ] Export issue data from all repos: `gh issue list --json --limit 1000`
- [ ] Note all existing tags (decree: v0.2.0, v0.3.1 + submodule tags; Python/TS: v0.1.0)
- [ ] Full local clones of all 4 repos as backups
- [ ] Decide on version numbers (v0.3.0 for decree, v0.2.0 for satellites)

### Phase 1: Create org and workspace (manual — user does this)

- [ ] Create `opendecree` GitHub org (free plan)
- [ ] Set org description, avatar
- [ ] Create 4 empty repos (no README): `decree`, `decree-python`, `decree-typescript`, `decree-ui`
- [ ] Create `.github` repo for org-level community health files
- [ ] Create org-level GitHub Project board, note its URL
- [ ] Add org-level secrets: `BUF_TOKEN`, `PROJECT_TOKEN`
- [ ] Create environments: `pypi` in decree-python, `npm` in decree-typescript
- [ ] Create local workspace: `mkdir ~/decree-workspace`

### Phase 2: Prepare decree repo (local, all changes before push)

**Step 2.1 — Global find-replace (mechanical)**

```bash
# Primary: Go module paths (covers go.mod, go.sum, .go, go.work)
find . -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' -o -name 'go.work' \) \
  -exec sed -i 's|github.com/opendecree/decree|github.com/opendecree/decree|g' {} +

# Secondary: all other files (YAML, JSON, MD, Makefile, Dockerfile, etc.)
find . -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name '*.md' \
  -o -name 'Makefile' -o -name 'Dockerfile*' -o -name '*.toml' -o -name '*.txt' \) \
  -exec sed -i 's|github.com/opendecree/decree|github.com/opendecree/decree|g' {} +
```

**Step 2.2 — Targeted replaces (non-module paths)**

```bash
# Badge URLs, repo_name, short references (without github.com/ prefix)
find . -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' \) \
  -exec sed -i 's|opendecree/decree|opendecree/decree|g' {} +

# Docker registry (hardcoded in Helm, docs, checklists)
find . -type f -exec sed -i 's|ghcr.io/opendecree/|ghcr.io/opendecree/|g' {} +

# GitHub project board URL
find . -type f -name '*.yml' \
  -exec sed -i 's|users/zeevdr/projects/2|orgs/opendecree/projects/N|g' {} +
# (Replace N with actual project number after Phase 1)

# Claude skills owner
find .claude/ -type f -exec sed -i 's|owner zeevdr|owner opendecree|g' {} +

# Governance email → GitHub-native
# CODE_OF_CONDUCT.md: replace conduct@zeevdr.dev with GitHub Issues link
# SECURITY.md: replace security@zeevdr.dev with GitHub Security Advisories link

# Helm maintainer
# Chart.yaml: update maintainer name and URL

# mkdocs repo_name
sed -i 's|repo_name: opendecree/decree|repo_name: opendecree/decree|' mkdocs.yml

# JSON schema $id
sed -i 's|zeevdr|opendecree|' schemas/schema-yaml.json
```

**Step 2.3 — Regenerate and validate**

```bash
# Regenerate proto stubs (picks up new go_package_prefix from buf.gen.yaml)
make generate

# Tidy all modules (dependency order)
for mod in api sdk/configclient sdk/adminclient sdk/configwatcher sdk/tools cmd/decree e2e . \
  examples/quickstart examples/setup examples/feature-flags examples/live-config \
  examples/schema-lifecycle examples/multi-tenant examples/optimistic-concurrency \
  examples/environment-bootstrap examples/config-validation; do
  (cd "$mod" && go mod tidy)
done

# Build
make build

# Test
make test

# Lint
make lint

# FINAL VERIFICATION — must return zero hits
grep -r "zeevdr" . \
  --include='*.go' --include='*.mod' --include='*.yaml' --include='*.yml' \
  --include='*.json' --include='*.md' --include='*.toml' --include='Makefile' \
  --include='Dockerfile*' --include='*.txt'
```

### Phase 3: Prepare satellite repos (local)

For each of decree-python, decree-typescript, decree-ui:

```bash
# Global replace
find . -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' -o -name '*.json' \
  -o -name '*.toml' \) -exec sed -i 's|opendecree/decree|opendecree/decree|g' {} +

# Project board URL
sed -i 's|users/zeevdr/projects/2|orgs/opendecree/projects/N|' .github/workflows/project.yml

# Governance emails → GitHub-native
# (same approach as decree)

# Verify
grep -r "zeevdr" .
```

Per-repo validation:
- **decree-python**: `make pre-commit`
- **decree-typescript**: `npm run pre-commit`
- **decree-ui**: `npm run pre-commit`

### Phase 4: Squash, push, and clone into workspace

For each repo:

```bash
# Squash and push
git checkout --orphan fresh
git add -A
git commit -m "Initial commit: OpenDecree <version>

Migrated from github.com/zeevdr/<repo> with clean history.
All paths updated to github.com/opendecree/<repo>."

git remote add org https://github.com/opendecree/<repo>.git
git push org fresh:main
```

Then set up the local workspace:

```bash
cd ~/decree-workspace
git clone https://github.com/opendecree/decree.git
git clone https://github.com/opendecree/decree-python.git
git clone https://github.com/opendecree/decree-typescript.git
git clone https://github.com/opendecree/decree-ui.git
```

Create workspace-level `~/decree-workspace/CLAUDE.md` with cross-repo context.
Move/recreate `.claude/` workspace memory from old location.

### Phase 5: Configure new repos

- [ ] Branch protection on main (all 4 repos)
- [ ] Enable GitHub Pages (decree only)
- [ ] Enable Issues (all repos)
- [ ] Set default branch to main
- [ ] Add repo descriptions and topics

### Phase 6: Tag and release

**decree:**
```bash
git tag -a v0.3.0 -m "v0.3.0 — first release under opendecree org"
git tag -a api/v0.3.0 -m "api v0.3.0"
git tag -a sdk/configclient/v0.3.0 -m "sdk/configclient v0.3.0"
git tag -a sdk/adminclient/v0.3.0 -m "sdk/adminclient v0.3.0"
git tag -a sdk/configwatcher/v0.3.0 -m "sdk/configwatcher v0.3.0"
git tag -a sdk/tools/v0.3.0 -m "sdk/tools v0.3.0"
git tag -a cmd/decree/v0.3.0 -m "cmd/decree v0.3.0"
git push origin --tags
```

**Satellites:**
```bash
# decree-python, decree-typescript, decree-ui
git tag -a v0.2.0 -m "v0.2.0 — first release under opendecree org"
git push origin --tags
```

### Phase 7: Post-migration validation

- [ ] CI passes on all 4 repos
- [ ] `go install github.com/opendecree/decree/cmd/decree@v0.3.0` works
- [ ] `go get github.com/opendecree/decree/sdk/configclient@v0.3.0` works
- [ ] `docker pull ghcr.io/opendecree/decree:latest` works
- [ ] GitHub Pages docs deploy
- [ ] Release workflow creates GitHub Release with binaries
- [ ] BSR push succeeds (buf.build/opendecree/decree)
- [ ] PyPI publish works (test with pre-release first)
- [ ] npm publish works (test with pre-release first)
- [ ] Project board receives issues/PRs
- [ ] `grep -r "zeevdr"` returns nothing in any repo

### Phase 8: Update external services

- [ ] PyPI: update trusted publisher → `opendecree/decree-python`
- [ ] npmjs: update trusted publisher → `opendecree/decree-typescript`
- [ ] buf.build: verify push works from new org
- [ ] pkg.go.dev: verify modules appear (~30 min after tags)

### Phase 9: Archive old repos

- [ ] Add "moved to opendecree/*" notice to each old repo README
- [ ] Archive all zeevdr/* repos (Settings → Archive)
- [ ] Update GitHub profile pins

## Rollback plan

If something goes wrong mid-migration:
- Old repos are untouched until Phase 9 (archiving)
- Can always delete the new org repos and start over
- Local backups from Phase 0 are the safety net
- OIDC trusted publishers: don't remove old config until new one is verified

## Risk matrix

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Missed zeevdr reference | Low | Medium | grep verification at every phase |
| Go module proxy caching old paths | N/A | None | No external consumers, alpha |
| BSR push fails from new org | Medium | Low | Test before removing old config |
| PyPI/npm OIDC fails from new org | Medium | Medium | Test with pre-release before removing old publisher |
| Docker images unreachable | Low | Low | Dynamic `repository_owner`, auto-resolves |
| CI secrets missing in new org | Medium | Medium | Checklist, test before tagging |

## GitHub issues

- #169 — Create opendecree org
- #170 — Create fresh repos with clean history
- #171 — Update Go module paths
- #172 — Update Docker/registry paths
- #173 — Reconfigure CI/CD
- #174 — Update Python/TypeScript SDK metadata + trusted publishers
- #175 — Update all documentation
- #176 — Tag v0.3.0
- #177 — Archive old repos
- #178 — Post-migration validation
