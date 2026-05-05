# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in OpenDecree, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please [report it via GitHub Security Advisories](https://github.com/opendecree/decree/security/advisories/new) with:

1. A description of the vulnerability
2. Steps to reproduce
3. The potential impact
4. Any suggested fix (optional)

You should receive a response within 48 hours. We will work with you to understand and address the issue before any public disclosure.

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.10.x (alpha) | ✅ |
| < 0.10 | ❌ |

During alpha, only the latest minor release receives security backports. See [GitHub Releases](https://github.com/opendecree/decree/releases) for the current version.

## Artifact Attestations

All release binaries and Docker images are signed with [Sigstore](https://sigstore.dev) via GitHub Actions artifact attestations ([SLSA](https://slsa.dev) provenance).

### Verify a downloaded binary

```bash
gh attestation verify decree_linux_amd64.tar.gz --owner opendecree
```

Replace the filename with the archive you downloaded (e.g. `decree-server_darwin_arm64.tar.gz`).

### Verify a Docker image

```bash
gh attestation verify oci://ghcr.io/opendecree/decree:VERSION --owner opendecree
gh attestation verify oci://ghcr.io/opendecree/decree-cli:VERSION --owner opendecree
```

Replace `VERSION` with the release version (e.g. `0.10.0-alpha.1`).

## Tamper-Evident Audit Log

OpenDecree maintains a tamper-evident audit chain for all configuration and admin mutations.

- Each audit row stores a SHA-256 hash chaining it to the previous entry for the same tenant.
- A database trigger rejects UPDATE/DELETE on rows older than 60 seconds, preventing silent history rewrites.
- Operators can verify chain integrity with `decree audit verify --tenant <id>`.

The chain relies on hash chaining, not HMAC-keyed authentication. It is a deterrent against casual tampering; a compromised application credential can insert false entries that appear valid in the chain. See `docs/development/threat-model.md` for the full trust model.

## Scope

This policy covers the OpenDecree server, CLI, and SDK packages in this repository.
