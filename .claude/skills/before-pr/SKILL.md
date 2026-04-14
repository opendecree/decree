---
name: before-pr
description: Run the before-PR checklist — rebase, docs, coverage, agent context, TODO scan. Run after /pre-commit passes.
user-invocable: true
allowed-tools: Bash(git *), Bash(gh *), Bash(make *), Bash(go *), Bash(./scripts/*), Glob, Grep, Read
---

Run the "Before PR" checklist from `docs/development/checklists.md`. Assumes `/pre-commit` already passed — skip those checks.

## Batch 1 (parallel — fast checks)

- **Rebase**: `git fetch origin main && git merge-base --is-ancestor origin/main HEAD` — if not, warn "branch needs rebase"
- **TODO/FIXME**: `grep -rn 'TODO\|FIXME' --include='*.go' --include='*.proto' --include='*.sql'` on changed files only (`git diff --name-only main..HEAD`) — flag any without a `#NNN` issue reference
- **Env vars**: check `git diff main..HEAD -- '*.go'` for new `os.Getenv` or `DECREE_` references — if found, verify they're in `docs/server/configuration.md`

## Batch 2 (parallel — generated code + docs)

- **Docs**: `make docs` — check `git diff --stat` after; if dirty, warn "docs need regenerating"
- **OpenAPI**: compare `cmd/server/openapi.json` and `docs/api/openapi.swagger.json` timestamps/checksums — if they differ, warn

## Batch 3 (review checks)

- **Coverage**: run `./scripts/coverage.sh` and compare badge value to what's in README — if they differ, warn
- **Agent context**: check if any open milestone has issues being closed by this branch (`Closes #N` in commit messages) — if so, remind to update `.agents/context/` if relevant
- **CLI commands**: if any `cobra.Command` was added in the diff, verify it has `Short` and `Long` fields

## Report

Format as a checklist:
```
✓ Branch up to date with main
✓ No undocumented TODOs
✓ No new env vars (or all documented)
✓ Generated docs up to date
✓ OpenAPI in sync
✓ Coverage badge accurate
✓ Agent context reviewed
✓ CLI commands have descriptions
```

Flag any warnings with ✗ and details. If all pass, say "Ready for /pr".
