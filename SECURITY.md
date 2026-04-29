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
| latest  | Yes       |

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

## Scope

This policy covers the OpenDecree server, CLI, and SDK packages in this repository.
