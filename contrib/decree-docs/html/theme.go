package html

import (
	"fmt"
	"strings"
)

// tokens holds the values of the 10 --decree-* custom properties for one
// color scheme.
type tokens struct {
	bg, surface, ink, muted, line, chip, accent, warn, info, danger string
}

var lightTokens = tokens{
	bg: "#fbfcfd", surface: "#ffffff", ink: "#1c1e21", muted: "#6b7178",
	line: "#e4e7eb", chip: "#eef1f4", accent: "#2f64d8", warn: "#9a6212",
	info: "#0c6f80", danger: "#a3322e",
}

var darkTokens = tokens{
	bg: "#0f1419", surface: "#161b22", ink: "#e6e9ed", muted: "#8b939c",
	line: "#272d36", chip: "#1f2630", accent: "#6f9bff", warn: "#d8a24a",
	info: "#5cc4d6", danger: "#e0716c",
}

// declare renders t as a block of `--decree-*: value;` declarations.
func (t tokens) declare() string {
	return fmt.Sprintf(`--decree-bg: %s;
  --decree-surface: %s;
  --decree-ink: %s;
  --decree-muted: %s;
  --decree-line: %s;
  --decree-chip: %s;
  --decree-accent: %s;
  --decree-warn: %s;
  --decree-info: %s;
  --decree-danger: %s;`,
		t.bg, t.surface, t.ink, t.muted, t.line, t.chip, t.accent, t.warn, t.info, t.danger)
}

// buildCSS assembles the full inline stylesheet: the cascade-layer order
// declaration, the reset/theme/component layers, and (if userCSS is
// non-empty) a trailing decree.user layer wrapping it verbatim. Layer order
// — not source position — decides precedence, so decree.user always wins
// over decree.reset/theme/components regardless of selector specificity.
func buildCSS(theme Theme, userCSS string) string {
	var b strings.Builder

	fmt.Fprintln(&b, "@layer decree.reset, decree.theme, decree.components, decree.user;")
	b.WriteString(resetLayer)
	b.WriteString(themeLayer(theme))
	b.WriteString(componentsLayer)
	if strings.TrimSpace(userCSS) != "" {
		fmt.Fprintf(&b, "\n@layer decree.user {\n%s\n}\n", userCSS)
	}

	return b.String()
}

func themeLayer(theme Theme) string {
	switch theme {
	case Dark:
		return fmt.Sprintf(`
@layer decree.theme {
  :root {
    %s
  }
}
`, darkTokens.declare())
	case Auto:
		return fmt.Sprintf(`
@layer decree.theme {
  :root {
    %s
  }
  @media (prefers-color-scheme: dark) {
    :root {
      %s
    }
  }
}
`, lightTokens.declare(), darkTokens.declare())
	default: // Light
		return fmt.Sprintf(`
@layer decree.theme {
  :root {
    %s
  }
}
`, lightTokens.declare())
	}
}

const resetLayer = `
@layer decree.reset {
  *, *::before, *::after { box-sizing: border-box; }
  body { margin: 0; }
  a { color: inherit; }
}
`

// componentsLayer styles the page shell, nav, field cards, and badges using
// only --decree-* custom properties (with color-mix() deriving tinted
// backgrounds from the 10 core tokens, so no extra tokens are needed for
// badge fills) plus a system font stack — no external fonts/icons/scripts.
const componentsLayer = `
@layer decree.components {
  body {
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
    color: var(--decree-ink);
    background: var(--decree-bg);
  }
  code, .mono { font-family: ui-monospace, "SF Mono", Menlo, monospace; }

  .decree-doc { min-height: 100vh; }
  header.decree-header {
    display: flex; align-items: center; gap: 18px;
    padding: 14px 24px; border-bottom: 1px solid var(--decree-line);
    background: var(--decree-surface);
  }
  header.decree-header h1 { font-size: 17px; font-weight: 650; letter-spacing: -.01em; margin: 0; }
  header.decree-header .sub { font-size: 13px; color: var(--decree-muted); margin-top: 2px; }

  .decree-body { display: flex; align-items: stretch; }
  nav.decree-nav {
    width: 256px; flex: none; align-self: stretch;
    border-right: 1px solid var(--decree-line); background: var(--decree-surface);
    padding: 18px 14px;
  }
  nav.decree-nav .group-title {
    font-size: 11px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase;
    color: var(--decree-muted); margin: 14px 0 8px;
  }
  nav.decree-nav .group-title:first-child { margin-top: 0; }
  nav.decree-nav a {
    display: block; padding: 6px 11px; border-radius: 7px; text-decoration: none;
    font-size: 13.5px; color: var(--decree-ink);
  }
  nav.decree-nav a.deprecated { color: var(--decree-muted); text-decoration: line-through; }
  nav.decree-nav .footer {
    margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--decree-line);
    font-size: 12px; color: var(--decree-muted); line-height: 1.5;
  }

  main.decree-main { flex: 1; min-width: 0; padding: 30px 40px 80px; max-width: 880px; }
  main.decree-main h2.group { font-size: 26px; font-weight: 700; letter-spacing: -.02em; margin: 32px 0 18px; }
  main.decree-main h2.group:first-child { margin-top: 8px; }

  section.field-card {
    border: 1px solid var(--decree-line); border-radius: 12px; background: var(--decree-surface);
    padding: 18px 20px; margin-bottom: 18px; scroll-margin-top: 20px;
  }
  section.field-card .field-head { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; }
  section.field-card .field-head code.path { font-size: 16px; font-weight: 650; }
  section.field-card .field-head code.path.deprecated { text-decoration: line-through; color: var(--decree-muted); }
  section.field-card .type-chip {
    font-size: 11.5px; background: var(--decree-chip); color: var(--decree-ink);
    padding: 2px 8px; border-radius: 6px; font-weight: 600;
  }
  section.field-card .desc { font-size: 14.5px; margin: 11px 0 12px; line-height: 1.55; }
  section.field-card .meta-row { display: flex; gap: 18px; flex-wrap: wrap; font-size: 13px; color: var(--decree-muted); margin-bottom: 12px; }
  section.field-card .default-chip {
    font-size: 12.5px; font-weight: 600; color: var(--decree-accent); padding: 2px 8px; border-radius: 5px;
    background: color-mix(in srgb, var(--decree-accent) 12%, var(--decree-surface));
  }
  section.field-card .tag-chip { font-size: 12px; background: var(--decree-chip); padding: 1px 7px; border-radius: 5px; margin-right: 4px; }

  .badge {
    display: inline-flex; align-items: center; gap: 4px; padding: 2px 9px; border-radius: 999px;
    font-size: 11px; font-weight: 650; letter-spacing: .03em; text-transform: uppercase;
  }
  .badge.neutral { background: var(--decree-chip); color: var(--decree-muted); }
  .badge.warn { color: var(--decree-warn); background: color-mix(in srgb, var(--decree-warn) 16%, var(--decree-surface)); }
  .badge.info { color: var(--decree-info); background: color-mix(in srgb, var(--decree-info) 16%, var(--decree-surface)); }
  .badge.danger { color: var(--decree-danger); background: color-mix(in srgb, var(--decree-danger) 16%, var(--decree-surface)); }
  .badge-row { display: flex; gap: 7px; flex-wrap: wrap; margin: 9px 0; }

  .deprecation-notice {
    display: flex; align-items: flex-start; gap: 9px; border-radius: 9px; padding: 10px 13px; margin: 13px 0;
    font-size: 13.5px; color: var(--decree-warn);
    background: color-mix(in srgb, var(--decree-warn) 10%, var(--decree-surface));
    border: 1px solid color-mix(in srgb, var(--decree-warn) 30%, var(--decree-surface));
  }
  .deprecation-notice code {
    background: var(--decree-surface); color: var(--decree-accent); padding: 1px 7px; border-radius: 5px;
  }

  .examples { margin: 14px 0; }
  .examples .label, .constraints .label {
    font-size: 11px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase;
    color: var(--decree-muted); margin-bottom: 7px;
  }
  .example-item {
    border: 1px solid var(--decree-line); border-radius: 9px; padding: 10px 14px; margin-bottom: 8px;
  }
  .example-item .name { font-size: 12px; font-weight: 650; color: var(--decree-muted); text-transform: uppercase; letter-spacing: .04em; margin-bottom: 4px; }
  .example-item code.value { background: var(--decree-chip); padding: 2px 8px; border-radius: 5px; }
  .example-item .summary { font-size: 13px; color: var(--decree-muted); margin-top: 6px; }

  .constraints ul { margin: 0; padding-left: 0; list-style: none; display: flex; flex-direction: column; gap: 5px; }
  .constraints li { font-size: 13.5px; }
  .constraints code { background: var(--decree-chip); padding: 1px 7px; border-radius: 5px; }

  .external-docs { margin-top: 12px; font-size: 13px; }
  .external-docs a { color: var(--decree-accent); text-decoration: none; font-weight: 600; }

  .sensitive-mask {
    display: flex; align-items: center; gap: 9px; border: 1px solid var(--decree-line); border-radius: 8px;
    padding: 9px 13px; background: var(--decree-chip); font-family: ui-monospace, Menlo, monospace;
    font-size: 14px; color: var(--decree-muted); margin-top: 12px;
  }
  .sensitive-mask .note { margin-left: auto; font-size: 11px; font-family: ui-sans-serif, system-ui, sans-serif; }

  .validations { padding: 18px 40px 0; max-width: 880px; }
  .validations h2.group { font-size: 26px; font-weight: 700; letter-spacing: -.02em; margin: 8px 0 18px; }
  .validation-item {
    border-radius: 9px; padding: 12px 16px; margin-bottom: 12px;
    border: 1px solid color-mix(in srgb, var(--decree-danger) 30%, var(--decree-surface));
    background: color-mix(in srgb, var(--decree-danger) 8%, var(--decree-surface));
  }
  .validation-item.warning {
    border-color: color-mix(in srgb, var(--decree-warn) 30%, var(--decree-surface));
    background: color-mix(in srgb, var(--decree-warn) 8%, var(--decree-surface));
  }
  .validation-item .validation-head {
    display: flex; align-items: center; gap: 7px; margin-bottom: 8px;
    font-size: 11px; font-weight: 700; letter-spacing: .06em; text-transform: uppercase;
    color: var(--decree-danger);
  }
  .validation-item.warning .validation-head { color: var(--decree-warn); }
  .validation-item pre {
    margin: 0 0 8px; padding: 9px 12px; border-radius: 7px; overflow-x: auto;
    background: var(--decree-surface); border: 1px solid var(--decree-line);
  }
  .validation-item pre code { font-size: 13px; color: var(--decree-ink); }
  .validation-item .validation-message { font-size: 13.5px; margin: 0; }
}
`
