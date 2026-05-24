#!/usr/bin/env bash
# Enforce CREATE INDEX CONCURRENTLY convention in goose migration files.
#
# Rules:
#   1. Any file containing CREATE INDEX CONCURRENTLY must have -- +goose NO TRANSACTION.
#   2. Any file containing CREATE INDEX (blocking) must annotate each such line with
#      -- decree:index-lock-ok <reason> or fail the check.
#
# Usage: ./scripts/check-concurrent-indexes.sh [migrations-dir]
#   Default migrations-dir: db/migrations

set -euo pipefail

MIGRATIONS_DIR="${1:-db/migrations}"
ERRORS=0

while IFS= read -r -d '' f; do
    # Check rule 1: CONCURRENTLY without NO TRANSACTION header.
    if grep -qiE 'CREATE (UNIQUE )?INDEX CONCURRENTLY' "$f"; then
        if ! grep -q '^\-\- +goose NO TRANSACTION' "$f"; then
            echo "ERROR: $f uses CREATE INDEX CONCURRENTLY but is missing '-- +goose NO TRANSACTION'" >&2
            ERRORS=$((ERRORS + 1))
        fi
    fi

    # Check rule 2: blocking CREATE INDEX without suppression annotation.
    while IFS= read -r line; do
        if echo "$line" | grep -qiE 'CREATE (UNIQUE )?INDEX [^C]' && \
           ! echo "$line" | grep -qiE 'CREATE (UNIQUE )?INDEX CONCURRENTLY'; then
            if ! echo "$line" | grep -q 'decree:index-lock-ok'; then
                echo "ERROR: $f: blocking CREATE INDEX missing annotation (add -- decree:index-lock-ok <reason> or use CONCURRENTLY):" >&2
                echo "  $line" >&2
                ERRORS=$((ERRORS + 1))
            fi
        fi
    done < "$f"
done < <(find "$MIGRATIONS_DIR" -name '*.sql' -print0 | sort -z)

if [[ $ERRORS -gt 0 ]]; then
    echo "" >&2
    echo "Fix: use CREATE INDEX CONCURRENTLY with '-- +goose NO TRANSACTION', or add '-- decree:index-lock-ok <reason>' for intentional blocking indexes." >&2
    exit 1
fi

echo "check-concurrent-indexes: OK ($(find "$MIGRATIONS_DIR" -name '*.sql' | wc -l | tr -d ' ') migrations checked)"
