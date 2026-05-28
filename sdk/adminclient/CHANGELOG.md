# Changelog

All notable changes to `github.com/opendecree/decree/sdk/adminclient` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses semantic versioning with an `-alpha.N` suffix during alpha; the API is unstable and breaking changes ship without backward-compatibility shims.

## [Unreleased]

### Added

- `Field.Type` is now typed as `FieldType` enum instead of `string`
- Explicit-page list APIs, streaming iterator, and chunked `VerifyChain` support (#602)
- `WithRetry` option and `RetryableError` sentinel mapping on `AdminClient` (#583)
- Tamper-evident SHA-256 hash chain on audit records

### Changed

- `New` migrated to `With...()` functional options — positional variadic-optional args removed (#584)

## [0.11.0-alpha.1] - 2026-05-03

### Changed

- `adminclient.New` migrated to `With...()` functional options; positional `Config` struct removed (#249)
  - Old: `adminclient.New(nil, nil, ma, nil)`
  - New: `adminclient.New(adminclient.WithAuditTransport(ma))`
  - Options: `WithSchemaTransport`, `WithConfigTransport`, `WithAuditTransport`, `WithServerTransport`

## [0.10.0-alpha.1] - 2026-04-27

First tracked release.

[Unreleased]: https://github.com/opendecree/decree/compare/sdk/adminclient/v0.11.0-alpha.1...HEAD
[0.11.0-alpha.1]: https://github.com/opendecree/decree/compare/sdk/adminclient/v0.10.0-alpha.1...sdk/adminclient/v0.11.0-alpha.1
[0.10.0-alpha.1]: https://github.com/opendecree/decree/releases/tag/sdk/adminclient/v0.10.0-alpha.1
