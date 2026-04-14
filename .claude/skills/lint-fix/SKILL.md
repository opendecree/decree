---
name: lint-fix
description: Run linters and auto-fix all issues. Detects repo type (Go or JS/TS) and iterates until clean.
user-invocable: true
allowed-tools: Bash(gofumpt *), Bash(golangci-lint *), Bash(go vet *), Bash(npx biome *), Bash(npx tsc *), Glob
---

Run the full lint suite, fix all issues, and iterate until clean.

## Auto-detect repo type

Check for `biome.json` in the current directory:
- **If found** → JavaScript/TypeScript repo (biome)
- **If not found** → Go repo (gofumpt + golangci-lint + go vet)

## Go repo

1. `gofumpt -l .` — if files listed, run `gofumpt -w` on each
2. `golangci-lint run ./...` — fix reported issues manually
3. `go vet ./...` — fix reported issues
4. Repeat steps 1-3 until all pass with no output

## JavaScript/TypeScript repo (biome)

1. `npx biome check --write src/` — auto-fix what biome can
2. If errors remain after `--write`, apply `--unsafe` fixes: `npx biome check --write --unsafe src/`
3. If errors still remain, fix them manually
4. `npx tsc --noEmit` — fix type errors
5. Repeat until clean

## Report

```
✓ All clean (N files auto-fixed)
```

Or if zero issues: `✓ Already clean`
