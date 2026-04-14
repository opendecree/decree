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
4. Create the PR:

```
gh pr create --title "TITLE" --body "$(cat <<'EOF'
## Summary
<1-3 bullet points summarizing the changes>

## Test plan
<bulleted checklist of what was tested>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

5. Report the PR URL

## Rules
- Do NOT assign reviewers (solo maintainer)
- Do NOT set milestone (managed separately)
- Title under 70 characters
- Summary bullets focus on "why" not "what"
- Include issue references (Closes #N) when applicable
