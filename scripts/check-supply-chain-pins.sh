#!/usr/bin/env bash
# check-supply-chain-pins.sh — Enforce supply-chain pinning policy.
#
# 1. Every `uses:` line in .github/workflows/*.yml that references a
#    non-trusted action must pin to a 40-char commit SHA.
#    Trusted orgs (allowed to pin by tag): actions, github, docker.
# 2. Every `FROM` line in build/* must pin to an `@sha256:<digest>`.
#
# Exits non-zero with a list of offenders.
set -euo pipefail

cd "$(dirname "$0")/.."

fail=0

# --- workflows ---------------------------------------------------------------
# Match `uses: <ref>` (ignore lines inside comments). Skip trusted orgs.
# A pinned ref is `<owner>/<repo>[/path]@<40-hex>`.
while IFS= read -r line; do
  # Strip leading "<file>:<n>:" prefix from grep -n -H
  file_loc="${line%%:*}"
  rest="${line#*:}"
  lineno="${rest%%:*}"
  content="${rest#*:}"
  # Trim leading whitespace
  trimmed="${content#"${content%%[![:space:]]*}"}"
  # Skip comment lines
  [[ "$trimmed" == \#* ]] && continue
  # Strip optional list-item prefix `- `
  trimmed="${trimmed#- }"
  trimmed="${trimmed#"${trimmed%%[![:space:]]*}"}"
  # Extract the ref after `uses:`
  ref="${trimmed#uses:}"
  ref="${ref#"${ref%%[![:space:]]*}"}"
  # Skip local workflow refs
  [[ "$ref" == ./* ]] && continue
  # Trusted orgs may continue to pin by tag
  case "$ref" in
    actions/*|github/*|docker/*) continue ;;
  esac
  # Ref must look like owner/repo[/path]@<sha>
  sha="${ref##*@}"
  if [[ ! "$sha" =~ ^[0-9a-f]{40}$ ]]; then
    echo "non-trusted action not SHA-pinned: $file_loc:$lineno: $ref" >&2
    fail=1
  fi
done < <(grep -rHnE '^[[:space:]]*-?[[:space:]]*uses:[[:space:]]' .github/workflows/ || true)

# --- Dockerfiles -------------------------------------------------------------
while IFS= read -r line; do
  file_loc="${line%%:*}"
  rest="${line#*:}"
  lineno="${rest%%:*}"
  content="${rest#*:}"
  # Extract image ref after FROM (drop `AS <name>` suffix)
  ref="${content#FROM }"
  ref="${ref%% AS *}"
  ref="${ref%% as *}"
  if [[ "$ref" != *"@sha256:"* ]]; then
    echo "Dockerfile FROM not digest-pinned: $file_loc:$lineno: $content" >&2
    fail=1
  fi
done < <(grep -rHnE '^FROM ' build/ || true)

if [[ $fail -ne 0 ]]; then
  echo "" >&2
  echo "See docs/development/checklists.md for the supply-chain pinning policy." >&2
  exit 1
fi

echo "supply-chain pins OK"
