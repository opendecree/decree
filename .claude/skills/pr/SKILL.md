---
name: pr
description: Create a pull request with standard template. Reads commits, determines title and labels, pushes if needed.
user-invocable: true
argument-hint: [optional title override]
allowed-tools: Bash(git *), Bash(gh pr *)
---

Create a pull request for the current branch.

## Steps

1. Ensure branch has an upstream: if `git rev-parse --abbrev-ref @{upstream}` fails, run `git push -u origin HEAD`
2. Read commits: `git log --oneline main..HEAD`
3. Determine title:
   - If `$ARGUMENTS` is provided, use it as the title
   - Otherwise, derive from commits (under 70 chars, imperative mood)
4. Detect issue references by running `check.sh` from the skill directory:

```bash
eval "$(bash "$(dirname "$0")/check.sh" 2>/dev/null)" || true
```

This sets `$ISSUE_REFS` (space-separated issue numbers, e.g. `"312 456"`).

5. Branch on issue refs:

   **If `$ISSUE_REFS` is non-empty:**
   - Build a `Closes` block from the refs:
     ```
     Closes #312
     Closes #456
     ```
   - Include this block at the top of the PR body (before `## Summary`).

   **If `$ISSUE_REFS` is empty:**
   - Print a warning:
     ```
     Warning: This PR does not reference any issue. Open anyway, or attach an issue?
     Enter issue number to attach (or press Enter to open without one):
     ```
   - Read user input. If they provide a number N, set `ISSUE_REFS="N"` and include `Closes #N` in the body. If they press Enter (empty input), proceed without a `Closes` line.

6. Create the PR. When refs are present, include the `Closes` block:

```
gh pr create --title "TITLE" --body "$(cat <<'EOF'
Closes #NNN

## Summary
<1-3 bullet points summarizing the changes>

## Test plan
<bulleted checklist of what was tested>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

When no refs, omit the `Closes` line.

7. Report the PR URL

## Rules
- Do NOT assign reviewers (solo maintainer)
- Do NOT set milestone (managed separately)
- Title under 70 characters
- Summary bullets focus on "why" not "what"
- Always include `Closes #N` when issue refs are found in commits
