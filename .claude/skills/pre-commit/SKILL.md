---
name: pre-commit
description: Run the before-commit checklist — build, lint, format, test, coverage. Reports pass/fail.
user-invocable: true
allowed-tools: Bash(make *), Bash(gofumpt *), Bash(git diff *), Glob, Grep, Read
---

Run the before-commit checklist using `make pre-commit`.

## Steps

1. Run `make pre-commit` — this executes build, vet, format, lint, test, and coverage across all modules
2. If format check fails, run `gofumpt -w .` to auto-fix, then re-run
3. After `make pre-commit` passes, run safety checks:
   - Check staged files for secrets: scan `git diff --cached --name-only` output for patterns like `.env`, `.pem`, `.key`, `credentials`, `secret`, `token` in filenames
   - Check for binary files: `git diff --cached --numstat | grep '^-'` (binary files show `-` for lines)

## Report

Format results as a checklist:
```
✓ Build clean
✓ Vet clean
✓ Format clean
✓ Lint clean
✓ Tests pass (all modules)
✓ Coverage ratchet pass
✓ No secrets detected
✓ No binary files staged
```

If ANY check fails, report the failure details and DO NOT proceed with the commit.
