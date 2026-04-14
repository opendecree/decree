---
name: squash
description: Squash commits on the current branch before merge. Shows plan and asks for confirmation.
user-invocable: true
allowed-tools: Bash(git *)
---

Squash commits on the current branch before merge.

## Steps

1. Run `git log --oneline main..HEAD` to see all commits
2. If only 1 commit, say "nothing to squash" and stop
3. Show the commit list and proposed squash strategy:
   - **Group related commits** — a fix + its follow-up = one commit
   - **Keep distinct features/fixes as separate commits** — don't flatten everything
   - Preserve all Co-Authored-By lines
4. **Ask for confirmation before executing** — this is destructive
5. Execute: `git reset --soft $(git merge-base HEAD main)` then create the new commit(s)
6. If the branch has been pushed, `git push --force-with-lease`
7. Report the result: old commit count → new commit count

## Rules
- NEVER squash without showing the plan first
- NEVER squash without user confirmation
- Group logically — not everything into one commit unless that makes sense
- Always preserve Co-Authored-By lines in the final commit(s)
