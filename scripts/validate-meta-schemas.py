#!/usr/bin/env python3
"""validate-meta-schemas: assert that meta-schemas accept what they should
and reject what they should.

For every canonical schema/config YAML in the repo, the matching meta-schema
must accept it. For every fixture under schemas/v0.1.0/testdata/invalid/,
the meta-schema must REJECT it (proves the meta-schema isn't permissive
enough to leak malformed input through).

Used by CI (issue #124) and by `make validate-meta-schemas` for local
sanity checks. Exits 0 on success; nonzero with a printed report listing
each failed assertion otherwise.

Vanilla — depends only on PyYAML and jsonschema, both already available
in the project's tools image (and used by scripts/patch-openapi.py).
"""

from __future__ import annotations

import glob
import json
import sys
from pathlib import Path

import jsonschema
import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent
SCHEMAS_DIR = REPO_ROOT / "schemas" / "v0.1.0"

# Globs of canonical files that must validate cleanly against the named
# meta-schema. Fixtures under schemas/v0.1.0/testdata/invalid/ are handled
# separately as negative-cases.
_ROOTS = ("examples", "e2e", "docs")
# Match both the canonical bare names ("decree.schema.yaml",
# "decree.config.yaml") and the prefixed-glob form authors use when a
# repo holds multiple schemas (`payments.decree.schema.yaml`).
SCHEMA_GOOD_GLOBS = [f"{r}/**/decree.schema.yaml" for r in _ROOTS] + [
    f"{r}/**/*.decree.schema.yaml" for r in _ROOTS
]
CONFIG_GOOD_GLOBS = [f"{r}/**/decree.config.yaml" for r in _ROOTS] + [
    f"{r}/**/*.decree.config.yaml" for r in _ROOTS
]


def load_meta(name: str) -> dict:
    with open(SCHEMAS_DIR / name) as f:
        return json.load(f)


def expand(globs: list[str]) -> list[Path]:
    out: list[Path] = []
    for g in globs:
        out.extend(Path(p) for p in glob.glob(str(REPO_ROOT / g), recursive=True))
    return sorted(out)


def validate_one(meta: dict, path: Path) -> list[str]:
    with open(path) as f:
        doc = yaml.safe_load(f)
    return [e.message for e in jsonschema.Draft202012Validator(meta).iter_errors(doc)]


def main() -> int:
    schema_meta = load_meta("decree-schema.json")
    config_meta = load_meta("decree-config.json")
    failures: list[str] = []

    # Positive cases: canonical files must validate cleanly.
    schema_files = expand(SCHEMA_GOOD_GLOBS)
    print(f"Validating {len(schema_files)} *.decree.schema.yaml against decree-schema.json")
    for path in schema_files:
        errs = validate_one(schema_meta, path)
        if errs:
            failures.append(f"{path.relative_to(REPO_ROOT)} expected pass but got {len(errs)} errors: {errs[0]}")

    config_files = expand(CONFIG_GOOD_GLOBS)
    print(f"Validating {len(config_files)} *.decree.config.yaml against decree-config.json")
    for path in config_files:
        # Skip files explicitly named "invalid.*" — those are intentionally
        # bad payloads used by the example to demonstrate validation errors;
        # they validate at the meta-schema layer (file shape is fine), but
        # the in-app validator rejects the values themselves.
        if path.name.startswith("invalid"):
            continue
        errs = validate_one(config_meta, path)
        if errs:
            failures.append(f"{path.relative_to(REPO_ROOT)} expected pass but got {len(errs)} errors: {errs[0]}")

    # Negative cases: fixtures under testdata/invalid/ must FAIL.
    for path in sorted((SCHEMAS_DIR / "testdata" / "invalid" / "schema").glob("*.yaml")):
        errs = validate_one(schema_meta, path)
        if not errs:
            failures.append(f"{path.relative_to(REPO_ROOT)} expected fail but passed validation")

    for path in sorted((SCHEMAS_DIR / "testdata" / "invalid" / "config").glob("*.yaml")):
        errs = validate_one(config_meta, path)
        if not errs:
            failures.append(f"{path.relative_to(REPO_ROOT)} expected fail but passed validation")

    if failures:
        print()
        print(f"FAIL: {len(failures)} assertion(s) failed:")
        for f in failures:
            print(f"  - {f}")
        return 1

    print("OK — all canonical files pass and all known-invalid fixtures fail.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
