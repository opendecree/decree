---
name: status
description: Show session status dashboard — git state, open PRs, CI, milestones, running services. Use at session start.
user-invocable: true
allowed-tools: Bash(git *), Bash(gh *), Bash(docker compose ps*), Bash(curl *), Read
---

Produce a concise session status dashboard. Run these in parallel where possible:

1. **Git**: current branch, uncommitted changes count, last 3 commits (oneline)
2. **Open PRs**: `gh pr list --state open --limit 5 --json number,title,headRefName,state`
3. **CI**: `gh run list --limit 1 --json status,conclusion,name,headBranch` for current branch
4. **Milestones**: `gh api repos/{owner}/{repo}/milestones --jq '.[] | "\(.title): \(.open_issues)/\(.open_issues + .closed_issues)"'`
5. **Services**: `docker compose ps --format json 2>/dev/null` (skip if nothing running)

Format as a compact dashboard with section headers. No prose — just the facts. Skip sections that return empty results.

Example output:
```
## Git
Branch: fix/some-bug | 2 uncommitted changes
- abc1234 Latest commit message
- def5678 Previous commit

## Open PRs
#127 Migrate efforts to milestones (chore/efforts-to-milestones)

## CI
✓ CI passed on fix/some-bug

## Milestones
Admin GUI: 4/6 | Security Review: 0/1 | SDK Examples: 0/4

## Services
postgres: healthy | redis: healthy | service: healthy
```
