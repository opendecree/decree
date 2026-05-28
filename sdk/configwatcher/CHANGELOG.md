# Changelog

All notable changes to `github.com/opendecree/decree/sdk/configwatcher` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses semantic versioning with an `-alpha.N` suffix during alpha; the API is unstable and breaking changes ship without backward-compatibility shims.

## [Unreleased]

### Added

- `WithReconnectTimeout` bounds the `loadSnapshot` call during reconnect with an explicit timeout (#410)
- Subscribe stream-end reconnect semantics documented in package godoc

### Fixed

- Dropped-value counter now surfaced instead of being silently discarded (#581)
- `Close`/channel-send race eliminated to prevent panic on shutdown (#577)
- Version gap between snapshot fetch and `Subscribe` stream eliminated (#500)

### Changed

- Default logger changed from `slog.Default()` to discard to avoid unintended log output in library consumers (#575)

## [0.11.0-alpha.1] - 2026-05-03

### Fixed

- Missing logger in direct `Watcher` construction during race tests (#354)

## [0.10.0-alpha.1] - 2026-04-27

Initial public release.

[Unreleased]: https://github.com/opendecree/decree/compare/sdk/configwatcher/v0.11.0-alpha.1...HEAD
[0.11.0-alpha.1]: https://github.com/opendecree/decree/compare/sdk/configwatcher/v0.10.0-alpha.1...sdk/configwatcher/v0.11.0-alpha.1
[0.10.0-alpha.1]: https://github.com/opendecree/decree/releases/tag/sdk/configwatcher/v0.10.0-alpha.1
