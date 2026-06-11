# contrib

Standalone tools built on top of decree. Each subdirectory is its own Go module, developed and versioned independently of the server and SDK modules.

> **Alpha**: These modules are part of the OpenDecree alpha release. The APIs may change.

## Scope: `contrib/` vs `sdk/contrib/`

- **`contrib/` (this directory)** hosts standalone tools — complete programs with their own CLIs that consume decree schemas or a running decree server (e.g. `decree-docs`).
- **[`sdk/contrib/`](../sdk/contrib/)** hosts SDK framework adapters — libraries that plug the Go SDK into third-party configuration frameworks (viper, koanf, envconfig).

## Modules

| Module | Description |
|--------|-------------|
| [`decree-docs`](decree-docs/) | Multi-format documentation generator for decree schemas |
