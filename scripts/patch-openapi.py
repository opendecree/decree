#!/usr/bin/env python3
"""Patch the generated OpenAPI spec with auth info and metadata.

Run after `make generate` to add security definitions, API info,
and alpha disclaimer to the OpenAPI spec.
"""

import json
import sys

SPEC_PATH = "docs/api/openapi.swagger.json"
COPY_PATH = "cmd/server/openapi.json"

patch = {
    "info": {
        "title": "OpenDecree API",
        "version": "0.4.0-alpha",
        "description": (
            "Schema-driven business configuration management for multi-tenant services.\n\n"
            "**Alpha** — APIs and behavior may change without notice between versions."
        ),
        "license": {
            "name": "Apache 2.0",
            "url": "https://www.apache.org/licenses/LICENSE-2.0",
        },
    },
    "securityDefinitions": {
        "MetadataAuth": {
            "type": "apiKey",
            "in": "header",
            "name": "x-subject",
            "description": (
                "Actor identity (required). Also set:\n"
                "- x-role: superadmin (default), admin, or user\n"
                "- x-tenant-id: required for non-superadmin, comma-separated for multi-tenant access"
            ),
        },
        "BearerAuth": {
            "type": "apiKey",
            "in": "header",
            "name": "Authorization",
            "description": (
                'JWT Bearer token (e.g., "Bearer eyJ...").\n'
                "The token's tenant_ids claim (array) determines accessible tenants."
            ),
        },
    },
    "security": [{"MetadataAuth": []}],
}


def main():
    with open(SPEC_PATH) as f:
        spec = json.load(f)

    spec["info"] = patch["info"]
    spec["securityDefinitions"] = patch["securityDefinitions"]
    spec["security"] = patch["security"]

    with open(SPEC_PATH, "w") as f:
        json.dump(spec, f, indent=2)

    with open(COPY_PATH, "w") as f:
        json.dump(spec, f, indent=2)

    print(f"Patched {SPEC_PATH} and {COPY_PATH}")
    print(f"  Title: {spec['info']['title']}")
    print(f"  Version: {spec['info']['version']}")
    print(f"  Security: {list(spec['securityDefinitions'].keys())}")


if __name__ == "__main__":
    main()
