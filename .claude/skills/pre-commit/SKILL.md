---
name: pre-commit
description: Run the before-commit checklist — build, lint, format, test, coverage, secrets scan. Reports pass/fail.
user-invocable: true
allowed-tools: Bash(go *), Bash(make *), Bash(gofumpt *), Bash(golangci-lint *), Bash(./scripts/*), Bash(grep *), Bash(git diff *), Glob, Grep, Read
---

Run the "Before Commit" checklist. Execute checks in parallel batches. Stop and report on first failure.

## Batch 1 (parallel — fast checks)
- `go build ./...` — must compile clean
- `go vet ./...` — must pass
- `gofumpt -l .` — must report no files (if it does, run `gofumpt -w` on them and report)

## Batch 2 (parallel — tests, only if batch 1 clean)
- `go test ./internal/... -count=1` — all tests pass
- `./scripts/check-coverage.sh` — coverage ratchet passes

## Batch 3 (parallel — safety checks)
- Check staged files for secrets: scan `git diff --cached --name-only` output for patterns like `.env`, `.pem`, `.key`, `credentials`, `secret`, `token` in filenames
- Check for binary files: `git diff --cached --numstat | grep '^-'` (binary files show `-` for lines)

## Report

Format results as a checklist:
```
✓ Build clean
✓ Vet clean
✓ Format clean (2 files auto-fixed)
✓ Tests pass (14 packages)
✓ Coverage ratchet pass
✓ No secrets detected
✓ No binary files staged
```

If ANY check fails, report the failure details and DO NOT proceed with the commit.
