---
name: merge
description: Merge a PR after checks pass — wait for CI, merge, switch to main, delete branch, verify, update context.
user-invocable: true
argument-hint: [PR number, default: current branch PR]
allowed-tools: Bash(git *), Bash(gh *), Read, Edit, Grep
---

Merge a pull request and run the "After PR Merge" checklist.

## Step 1 — Identify the PR

- If `$ARGUMENTS` is a number, use it as the PR number
- Otherwise, find the PR for the current branch: `gh pr view --json number --jq .number`
- If no PR found, stop with an error

## Step 2 — Wait for CI

- Run `gh pr checks <number> --watch` (timeout 5 minutes)
- If any check fails, report the failure and stop — do NOT merge

## Step 3 — Merge

- `gh pr merge <number> --squash --delete-branch`
- If merge fails (e.g. branch protection, conflicts), report and stop

## Step 4 — Local cleanup (parallel)

- `git checkout main && git pull --rebase`
- Delete local branch if it still exists: `git branch -d <branch> 2>/dev/null`

## Step 5 — Verify

- `gh run list --branch main --limit 1 --json status,conclusion,name` — report CI status on main
- Check if the PR closed any issues (`Closes #N` in PR body) — confirm they're now closed

## Step 6 — Agent context

- If a closed issue belongs to a milestone, check if the milestone is now fully closed
- If the work is significant (new feature, effort completion), remind to update `.agents/context/completed.md`
- If the milestone is fully closed, remind to close it and archive context

## Report

```
✓ PR #N merged (squash)
✓ Switched to main (abc1234)
✓ Branch deleted
✓ CI on main: passing
✓ Issue #M closed
→ Milestone "X": 3/4 complete (1 remaining)
```
