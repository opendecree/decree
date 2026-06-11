# Docs Toolkit — Design Context

Epic: #921 · Milestone: Docs Toolkit

## Goal

A standalone documentation generator for decree schemas — `decree-docs`, living in a new
top-level `contrib/` directory. Multiple output formats (json, md, mdx, html), built-in
themes, CSS style injection, and a template override system. The core `sdk/tools/docgen`
package stays minimal and zero-dependency; decree-docs is the full-featured tool built on
the same schema sources.

Design informed by a survey of Redoc/Redocly CLI, Swagger UI, Scalar, Stoplight Elements,
RapiDoc, Slate/Widdershins, json-schema-for-humans, Adobe jsonschema2md, terraform-docs,
helm-docs, and protoc-gen-doc.

## Non-goals

- Documenting this project's own site — the MkDocs site and opendecree.dev (#189) own
  that; decree-docs documents **user** schemas.
- Interactive try-it consoles (config schemas are not request/response APIs).
- Any dependency on an external site generator (the Widdershins→Slate trap; Slate was
  archived in 2026).

## Architecture

```
loader (file via sdk/tools/validate | server via sdk/adminclient)
  → doc model (complete: info, examples, externalDocs, version_description, allowed_schemes)
  → emitters: json | md (plain, material) | mdx (docusaurus) | html (single-file, themed)
templates: Go text/template in embed.FS, export-templates command, --template-files layering
config:    decree-docs.yaml with global + per-format sections; every key has a flag twin; flags win
```

## Key Decisions

1. **Top-level `contrib/`, not `sdk/contrib/`** — `contrib/` hosts standalone tools;
   `sdk/contrib/` hosts SDK framework adapters (viper/koanf/envconfig). Both READMEs state
   the split.
2. **Core docgen stays minimal** — `sdk/tools/docgen` remains the zero-dependency
   library; decree-docs composes on top rather than replacing it. Prerequisite fixes land
   in core first (#911 YAML metadata mapping, #912 online-mode parity).
3. **JSON doc model is a first-class output** (protoc-gen-doc pattern) — third-party
   renderers build on `--format json`, fulfilling the renderer goal stated in #117.
4. **Self-contained single-file HTML** (redocly build-docs pattern) — inline CSS, no
   server, no external assets; offline-renderable.
5. **Theming via CSS custom properties inside cascade layers** (Scalar pattern) —
   `--decree-*` variables, named themes (light/dark/auto), `--css <file>` user injection
   that wins without specificity wars.
6. **Templates: embed.FS + export + layering** (helm-docs pattern) — `export-templates`
   dumps built-ins for editing; repeatable `--template-files` layers overrides where later
   `define`s win.
7. **Config file with flag twins, flags win** (terraform-docs pattern) —
   `decree-docs.yaml`, global + per-format sections.
8. **Inject mode** (terraform-docs pattern) — write between
   `<!-- BEGIN_DECREE_DOCS -->`/`<!-- END_DECREE_DOCS -->` markers in existing files;
   idempotent.
9. **MDX v3 escaping is a correctness requirement** — `{` and `<` parse as JSX; all
   schema-sourced text must be escaped or code-spanned (top bug class in MDX generators).
10. **Go 1.24 tier, vanilla deps, untagged/unreleased at first** — same posture as the
    `sdk/contrib` modules; replace directives for intra-repo deps.

## Issue map

#911, #912 core docgen prerequisites → #913 scaffold → #914 doc model + loaders + JSON →
#915 md / #916 html / #917 mdx backends → #918 config + templates → #919 inject →
#920 docs + examples + freshness CI.
