# decree-docs

A standalone, multi-format documentation generator for decree schemas. It loads schemas from local files or from a running decree server and renders them as `json`, `md`, `mdx`, or `html`, with built-in themes, CSS style injection, and a template override system.

> **Alpha**: This module is part of the OpenDecree alpha release. The CLI and output formats may change.

> **Status**: scaffold — this build ships the command skeleton and version information only. The loaders and format backends land in upcoming releases.

`decree-docs` lives under `contrib/` (standalone tools built on decree), as opposed to `sdk/contrib/` (SDK framework adapters such as viper, koanf, and envconfig). It composes on top of the minimal, zero-dependency `sdk/tools/docgen` library rather than replacing it.

## Build

The module is not tagged for release yet; build it from a checkout:

```sh
git clone https://github.com/opendecree/decree
cd decree/contrib/decree-docs
go build -o decree-docs .
```

## Usage

```sh
decree-docs --help
decree-docs version
```
