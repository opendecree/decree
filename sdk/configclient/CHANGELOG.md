# Changelog

All notable changes to `github.com/opendecree/decree/sdk/configclient` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses semantic versioning with an `-alpha.N` suffix during alpha; the API is unstable and breaking changes ship without backward-compatibility shims.

## [Unreleased]

### Added

- `description`, `value_description`, and `expected_checksum` fields on write operation results (#606)

### Fixed

- Non-idempotent `Set*` operations no longer retried on transient errors (#595)
- `LockedValue.Set` correctly captures the client reference (#586)
- `TypedValue` accessors return `(T, bool)` instead of panicking on type mismatch (#499)
- Version gap between snapshot and `Subscribe` eliminated (#500)

### Changed

- `ErrPermissionDenied` and `ErrLocked` are separate sentinel errors (#582)

## [0.11.0-alpha.1] - 2026-05-03

First tracked release.

## [0.10.0-alpha.1] - 2026-04-27

Initial public release.

[Unreleased]: https://github.com/opendecree/decree/compare/sdk/configclient/v0.11.0-alpha.1...HEAD
[0.11.0-alpha.1]: https://github.com/opendecree/decree/compare/sdk/configclient/v0.10.0-alpha.1...sdk/configclient/v0.11.0-alpha.1
[0.10.0-alpha.1]: https://github.com/opendecree/decree/releases/tag/sdk/configclient/v0.10.0-alpha.1
