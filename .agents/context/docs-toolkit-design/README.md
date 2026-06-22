# decree-docs HTML/MDX design pass

Design artifacts for issues #916 (HTML backend) and #917 (MDX backend), per
the design-pass note on #916. Source: claude.ai/design project
`opendecree docs` (https://claude.ai/design/p/7b2bec14-2999-4f93-821c-0ff3a0932979).

- `01-wireframes.dc.html` — lo-fi pass. Three page-anatomy directions (2-pane,
  3-pane, compact list/table) and 6 field-card states. Open in a browser.
- `02-hifi.dc.html` — hi-fi pass. Locked direction: 2-pane is the only page
  anatomy, with a Card/Table density toggle in the same shell (3-pane rail
  dropped as a separate mode). Includes the `--decree-*` token table
  (light/dark, 10 tokens incl. `--decree-info` for write-once), the
  `--css` cascade-layer override demo, and the MDX content-layout spec
  (`_category_.json` / frontmatter / body).

## Locked decisions

1. Page anatomy: 2-pane (nav + content), examples render inline on the
   field card. Density toggle swaps content pane to a table view for large
   schemas — same header/nav shell, not a separate page layout.
2. Field-card content/order: badges → type/format → description →
   default/constraints/examples → deprecated/redirect last. 6 reference
   states cover every doc-model property at least once.
3. Theme tokens: `--decree-bg/surface/ink/muted/line/chip/accent/warn/info/danger`.
   `auto` = light at `:root`, dark via `@media (prefers-color-scheme:dark)`,
   same variable names.
4. `--css` injection: cascade layer order
   `decree.reset, decree.theme, decree.components, decree.user` — user file
   lands in `decree.user`, wins without `!important`.
5. MDX: one page per prefix group, `_category_.json` per folder, frontmatter
   carries id/title/sidebar_label/description/keywords, body is h2-per-field
   with Docusaurus `<Tabs>` for named examples and `:::caution` admonition
   for deprecated fields. Badges degrade to bold inline text (no MDX CSS).

Open either `.html` file directly in a browser to view/interact (theme and
density toggles are live).
