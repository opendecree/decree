---
name: issues
description: List open issues across all decree repos, grouped by size (S/M/L) and milestone. Quick view of what to tackle next.
user-invocable: true
allowed-tools: Bash(gh *)
---

List all open issues across the OpenDecree project, grouped by size label.

## Steps

1. Fetch all open issues from the project board:
   ```
   gh project item-list 2 --owner opendecree --limit 200 --format json
   ```
   Filter to items where `content.type == "Issue"` and status is not "Done".

2. Group by size label (`size: S`, `size: M`, `size: L`, unlabeled)
3. Within each size group, sort by repo then milestone (milestoned first, then unassigned)

## Output format

```
## size: S (quick wins)
decree#132     Add make pre-commit target                    [no milestone]
decree-ui#13   Add copyable ID attributes                   Admin GUI

## size: M (moderate)
decree#110     Stress test Phase 1                           Stress Testing
decree-python#14  CI hardening                               [no milestone]

## size: L (larger efforts)
decree#32      contrib/viper: remote config provider         Ecosystem
decree#88      Admin GUI Phase 8: embed in Go                Admin GUI

## Unlabeled
decree-ui#8    Add pre-commit npm script                     [no milestone]
```

Keep it compact — one line per issue, include repo prefix, right-aligned milestone. No prose, just the list.
