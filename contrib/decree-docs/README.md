# decree-docs

A standalone, multi-format documentation generator for decree schemas. It loads schemas from local files or from a running decree server and renders them as `json`, `md`, `mdx`, or `html`, with built-in themes, CSS style injection, and a template override system.

> **Alpha**: This module is part of the OpenDecree alpha release. The CLI and output formats may change.

> **Status**: in development — this build loads schemas from local YAML files and emits the json, md, and html documentation formats. The mdx backend, server mode, and the config/template system land in upcoming releases.

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
decree-docs generate --file decree.schema.yaml --format json
decree-docs generate --file decree.schema.yaml --format md --flavor material
decree-docs generate --file decree.schema.yaml --format md --pages multi --out-dir docs/
decree-docs generate --file decree.schema.yaml --format html --theme dark
decree-docs generate --file decree.schema.yaml --format html --css brand.css --out-dir docs/
decree-docs --help
decree-docs version
```

## The JSON doc model

`--format json` emits the tool's complete documentation model — a superset of the core `sdk/tools/docgen` schema covering every documented schema and field property: info (title, author, contact, labels), version descriptions, named examples, external documentation links, tags, format hints, read-only/write-once/sensitive flags, and all constraints including `allowedSchemes`. Third-party renderers can build on this output instead of parsing schema YAML themselves.

The shape is stable and versioned:

```json
{
  "docModelVersion": 1,
  "schema": {
    "name": "payments",
    "description": "Payment processing configuration.",
    "version": 3,
    "versionDescription": "Added refund window and webhook controls.",
    "info": {"title": "...", "author": "...", "contact": {}, "labels": {}},
    "fields": [
      {
        "path": "payments.fee",
        "type": "number",
        "description": "...",
        "default": "0.01",
        "examples": {"low": {"value": "0.01", "summary": "..."}},
        "externalDocs": {"description": "...", "url": "https://..."},
        "constraints": {"minimum": 0, "maximum": 1}
      }
    ]
  }
}
```

Serialization rules (see the `docmodel` package documentation for the authoritative reference):

- `docModelVersion` increments on breaking shape changes.
- All keys are lowerCamel (OpenAPI-style: `externalDocs`, `readOnly`, `exclusiveMinimum`).
- Optional empty values are omitted; absent and empty mean the same thing.
- Fields are sorted by path, so output is deterministic.
- Server-side bookkeeping (schema ID, checksum, published state, timestamps) is excluded: a schema loaded from a file and the same schema fetched from a server produce identical documents.

## The md format

`--format md` renders Markdown from the doc model. `--flavor plain` emits portable CommonMark; `--flavor material` additionally uses the `admonition` and `pymdownx.tabbed` extensions this repo's `mkdocs.yml` already enables — deprecation notices render as a `!!! warning` admonition, and fields with two or more named examples render as content tabs. In both flavors, deprecated/sensitive/read-only/write-once fields get a bold badge line distinct from the type/format meta line; material adds an icon per badge.

`--pages single` (default) renders one page: to stdout, or to `<out-dir>/index.md` with `--out-dir`. `--pages multi` renders an index page plus one page per top-level field-path-prefix group (e.g. `payments.fee` and `payments.retries` both land on `payments.md`) and requires `--out-dir`.

## The html format

`--format html` renders a single self-contained HTML document: inline CSS, no external assets, no network requests, viewable straight from disk. `--theme` selects a built-in color scheme — `light` (default), `dark`, or `auto` (light at `:root`, dark via `@media (prefers-color-scheme: dark)`, so it follows the reader's OS preference). All theme colors are exposed as 10 `--decree-*` CSS custom properties (`--decree-bg`, `--decree-surface`, `--decree-ink`, `--decree-muted`, `--decree-line`, `--decree-chip`, `--decree-accent`, `--decree-warn`, `--decree-info`, `--decree-danger`); badge/chip background tints derive from these via `color-mix()` rather than separate tokens.

`--css <file>` appends the file's contents inside a trailing `decree.user` CSS cascade layer (`@layer decree.reset, decree.theme, decree.components, decree.user;`). Cascade layer order — not selector specificity — decides precedence, so a one-line override like `:root { --decree-accent: #7c3aed; }` retints the whole document without `!important` or selector escalation.

The page anatomy (left nav grouped by field-path prefix, content pane of per-field cards with badges/examples/constraints) follows a design pass against OAS-reference-doc conventions (Redoc, Swagger UI, Scalar, Stoplight Elements); see `.agents/context/docs-toolkit-design/` for the locked wireframes/hi-fi mockups and token table.

`--format html` always renders to a single file: stdout, or `<out-dir>/index.html` with `--out-dir`.

## Testing

```sh
go test ./...
go test . -run TestGenerate_JSONGolden -update          # rewrite CLI JSON golden files
go test ./markdown -run TestRender_Golden -update       # rewrite markdown golden files
go test ./html -run TestRender_Golden -update           # rewrite html golden files
```
