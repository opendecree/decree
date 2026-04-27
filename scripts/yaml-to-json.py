#!/usr/bin/env python3
"""yaml-to-json: convert a YAML file to pretty-printed JSON.

Used to publish JSON copies of the meta-schemas under schemas/v0.1.0/
alongside their YAML sources. Consumers (schemastore.org, IDE
language servers, CI validators) typically prefer JSON; the YAML form
is the human-edited source of truth.

Usage:
    yaml-to-json.py <input.yaml> <output.json>

Vanilla — only depends on PyYAML, which is already in the project's
existing scripts/ tooling.
"""

import json
import sys

import yaml


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <input.yaml> <output.json>", file=sys.stderr)
        return 2
    src, dst = sys.argv[1], sys.argv[2]
    with open(src) as f:
        doc = yaml.safe_load(f)
    with open(dst, "w") as f:
        json.dump(doc, f, indent=2, sort_keys=False)
        f.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
