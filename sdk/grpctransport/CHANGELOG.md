# Changelog

All notable changes to `github.com/opendecree/decree/sdk/grpctransport` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses semantic versioning with an `-alpha.N` suffix during alpha; the API is unstable and breaking changes ship without backward-compatibility shims.

## [Unreleased]

### Added

- `WithTokenSource` option on `Dial` for per-RPC token refresh (#559)
- `Dial` now uses TLS by default; `WithInsecure` opt-out added

### Fixed

- gRPC keepalive enabled on dial to prevent silent connection stalls (#580)
- `ErrPermissionDenied` and `ErrLocked` are separate sentinel errors (#582)

### Changed

- Role header is now required; implicit superadmin default removed

## [0.11.0-alpha.1] - 2026-05-03

### Changed

- Dependency bumps (minor + patch)

## [0.10.0-alpha.1] - 2026-04-27

Initial public release.

[Unreleased]: https://github.com/opendecree/decree/compare/sdk/grpctransport/v0.11.0-alpha.1...HEAD
[0.11.0-alpha.1]: https://github.com/opendecree/decree/compare/sdk/grpctransport/v0.10.0-alpha.1...sdk/grpctransport/v0.11.0-alpha.1
[0.10.0-alpha.1]: https://github.com/opendecree/decree/releases/tag/sdk/grpctransport/v0.10.0-alpha.1
