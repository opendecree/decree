---
name: issue
description: Create a GitHub issue with proper labels, size, and milestone. Ensures consistent issue tracking.
user-invocable: true
argument-hint: <description of the issue>
allowed-tools: Bash(gh *)
---

Create a well-structured GitHub issue from a description.

## Steps

1. Parse `$ARGUMENTS` to understand the issue
2. Determine the **target repo** based on the issue's area:
   - `opendecree/decree` — server, CLI, Go SDKs, proto, infra, general
   - `opendecree/decree-python` — Python SDK
   - `opendecree/decree-typescript` — TypeScript SDK
   - `opendecree/decree-ui` — Admin GUI

3. Determine labels from the available set:

   **Type** (pick one):
   - `bug` — something broken
   - `enhancement` — new feature or improvement
   - `docs` — documentation only
   - `discovery` — research/design exploration

   **Area** (pick all that apply, decree repo only):
   - `server`, `sdk`, `cli`, `proto`, `ci`, `infra`, `docs`
   - `decree-ui`, `python-sdk`, `typescript-sdk`

   **Common labels** (all repos):
   - `ci` — CI/Infrastructure
   - `sdk` — SDK changes
   - `breaking` — breaking change

   **Size** (pick one):
   - `size: S` — quick win, a few hours or less
   - `size: M` — moderate, a day or two, clear scope
   - `size: L` — larger effort, multiple days, design decisions needed

4. Determine milestone — match to an open milestone if the issue fits one, otherwise leave unassigned
5. Show the plan and ask for confirmation before creating:

   ```
   Repo: opendecree/decree-ui
   Title: <title>
   Labels: enhancement, size: M
   Milestone: Admin GUI
   ---
   <body preview>
   ```

6. Create: `gh issue create --repo <repo> --title "..." --body "..." --label "..." --milestone "..."`
7. Report the issue URL

## Body format

```markdown
## Description
<1-3 sentences explaining the what and why>

## Acceptance criteria
- [ ] <concrete, testable items>
```

## Rules
- ALWAYS ask for confirmation before creating
- ALWAYS create in the correct repo based on the issue's area
- Do NOT assign (solo maintainer)
- Keep title under 70 characters
- Every issue gets a type label, area label (if applicable), and size label
