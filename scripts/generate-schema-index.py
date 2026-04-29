#!/usr/bin/env python3
"""generate-schema-index: build the Pages root index.html.

Walks <site>/schema/v*/ to discover published versions and writes a static
HTML page at <site>/index.html that lists each version's published files
with download links. Also writes <site>/robots.txt to keep crawlers off
the JSON (we don't need SEO for raw schema docs).

Usage:
    generate-schema-index.py <site-dir>

Vanilla — stdlib only. Called from .github/workflows/deploy-pages.yml.
"""

from __future__ import annotations

import html
import re
import sys
from pathlib import Path

VERSION_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")

PAGE_TEMPLATE = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>OpenDecree Meta-Schemas</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 720px; margin: 2rem auto; padding: 0 1rem; line-height: 1.5; color: #1f2937; }}
    h1 {{ font-size: 1.6rem; margin-bottom: 0.25rem; }}
    a {{ color: #2563eb; text-decoration: none; }}
    a:hover {{ text-decoration: underline; }}
    table {{ width: 100%; border-collapse: collapse; margin-top: 1rem; }}
    th, td {{ padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid #e5e7eb; }}
    th {{ font-weight: 600; background: #f9fafb; }}
    footer {{ margin-top: 2rem; font-size: 0.875rem; color: #6b7280; }}
  </style>
</head>
<body>
  <h1>OpenDecree Meta-Schemas</h1>
  <p>JSON Schema 2020-12 documents that describe the OpenDecree configuration format. Source of truth: <a href="https://github.com/opendecree/decree/tree/main/schemas">opendecree/decree</a>.</p>
  <table>
    <thead><tr><th>Version</th><th>File</th></tr></thead>
    <tbody>
{rows}
    </tbody>
  </table>
  <footer>Apache 2.0 · OpenDecree is alpha — all artifacts subject to change.</footer>
</body>
</html>
"""


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} <site-dir>", file=sys.stderr)
        return 2

    site = Path(sys.argv[1])
    schema_dir = site / "schema"
    if not schema_dir.is_dir():
        print(f"error: {schema_dir} not found", file=sys.stderr)
        return 1

    versions: list[tuple[tuple[int, int, int], str, list[str]]] = []
    for d in schema_dir.iterdir():
        if not d.is_dir():
            continue
        m = VERSION_RE.match(d.name)
        if not m:
            continue
        files = sorted(p.name for p in d.iterdir() if p.is_file())
        versions.append(((int(m.group(1)), int(m.group(2)), int(m.group(3))), d.name, files))
    versions.sort(reverse=True)

    rows: list[str] = []
    for _, ver, files in versions:
        for fname in files:
            rows.append(
                f'      <tr><td>{html.escape(ver)}</td>'
                f'<td><a href="schema/{html.escape(ver)}/{html.escape(fname)}">{html.escape(fname)}</a></td></tr>'
            )

    body = "\n".join(rows) or '      <tr><td colspan="2">No versions published yet.</td></tr>'
    (site / "index.html").write_text(PAGE_TEMPLATE.format(rows=body))
    (site / "robots.txt").write_text("User-agent: *\nDisallow: /\n")
    print(f"Wrote {site / 'index.html'} ({len(versions)} versions)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
