---
name: issues
description: List open issues across all decree repos, grouped by size (S/M/L) and milestone. Quick view of what to tackle next.
user-invocable: true
allowed-tools: Bash(gh *)
---

List all open issues across the OpenDecree project, grouped by size label.

## Steps

1. Fetch all open issues from the project board and display grouped by size:

```bash
gh project item-list 2 --owner opendecree --limit 200 --format json | python3 -c "
import json, sys

data = json.load(sys.stdin)
items = data.get('items', [])
issues = [i for i in items if i.get('content', {}).get('type') == 'Issue' and i.get('status') != 'Done']

groups = {'size: S': [], 'size: M': [], 'size: L': [], 'Unlabeled': []}
for item in issues:
    labels = item.get('labels') or []
    size_labels = []
    for l in labels:
        name = l if isinstance(l, str) else l.get('name', '')
        if name.startswith('size:'):
            size_labels.append(name)

    repo = item.get('content', {}).get('repository', '').split('/')[-1]
    number = item.get('content', {}).get('number', 0)
    title = item.get('content', {}).get('title', '')
    milestone = item.get('milestone') or ''
    if isinstance(milestone, dict):
        milestone = milestone.get('title', '')

    size = size_labels[0] if size_labels else 'Unlabeled'
    if size not in groups:
        size = 'Unlabeled'
    groups[size].append((repo, number, title, milestone))

for key in groups:
    groups[key].sort(key=lambda x: (x[0], x[3] == '', x[3], x[1]))

for label, display in [('size: S', 'size: S (quick wins)'), ('size: M', 'size: M (moderate)'), ('size: L', 'size: L (larger efforts)'), ('Unlabeled', 'Unlabeled')]:
    if not groups[label]:
        continue
    print(f'## {display}')
    for repo, num, title, ms in groups[label]:
        ref = f'{repo}#{num}'
        ms_display = ms if ms else '[no milestone]'
        print(f'{ref:<35} {title:<60} {ms_display}')
    print()
"
```

2. If there are unlabeled issues, mention them at the end so they can be sized.

Keep it compact — one line per issue, include repo prefix, right-aligned milestone. No prose, just the list.
