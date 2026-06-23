package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/contrib/decree-docs/html"
	"github.com/opendecree/decree/contrib/decree-docs/loader"
	"github.com/opendecree/decree/contrib/decree-docs/markdown"
	"github.com/opendecree/decree/contrib/decree-docs/mdx"
)

// docFormats lists the formats generate can emit.
var docFormats = []string{"json", "md", "html", "mdx"}

// mdFlavors lists the --flavor values valid with --format md.
var mdFlavors = []string{string(markdown.Plain), string(markdown.Material)}

// mdPageModes lists the --pages values valid with --format md.
var mdPageModes = []string{string(markdown.SinglePage), string(markdown.MultiPage)}

// htmlThemes lists the --theme values valid with --format html.
var htmlThemes = []string{string(html.Light), string(html.Dark), string(html.Auto)}

var generateCmd = &cobra.Command{
	Use:   "generate [schema-id]",
	Short: "Generate documentation from a decree schema",
	Long: `Generate documentation from a decree schema.

Provide --file to load a schema YAML file. Fetching a schema-id from a
running server arrives together with the server connection flags in an
upcoming release (#918); --file and schema-id are mutually exclusive.

The json format emits the complete documentation model: a stable JSON
document, versioned by its docModelVersion root marker, that third-party
renderers can build on.

The md format renders Markdown. --flavor plain emits portable CommonMark;
--flavor material additionally uses the MkDocs Material admonition and
content-tab extensions. --pages single (default) renders one page to
stdout (or to <out-dir>/index.md with --out-dir); --pages multi renders an
index page plus one page per top-level field group and requires --out-dir.

The html format renders a single self-contained HTML file (inline CSS, no
external assets, no network requests) to stdout or to <out-dir>/index.html
with --out-dir. --theme selects a built-in color scheme (light, dark, or
auto, which follows the reader's OS preference). --css <file> appends the
file's contents in a trailing CSS cascade layer, so user overrides take
precedence over the built-in theme without needing !important.

The mdx format renders a Docusaurus-compatible doc tree to <out-dir>: an
index.mdx overview page, plus one category folder per top-level field group,
each with a _category_.json (sidebar label and position) and an index.mdx.
--out-dir is required. Drop the tree into a Docusaurus docs/ folder.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().String("file", "", "schema YAML file (offline mode)")
	generateCmd.Flags().String("format", "json", "output format: "+strings.Join(docFormats, ", "))
	generateCmd.Flags().String("flavor", string(markdown.Plain), "md flavor: "+strings.Join(mdFlavors, ", "))
	generateCmd.Flags().String("pages", string(markdown.SinglePage), "md page mode: "+strings.Join(mdPageModes, ", "))
	generateCmd.Flags().String("out-dir", "", "output directory (required when md rendering produces multiple pages)")
	generateCmd.Flags().String("theme", string(html.Light), "html theme: "+strings.Join(htmlThemes, ", "))
	generateCmd.Flags().String("css", "", "html: file whose contents are appended as a user CSS override")
	_ = generateCmd.RegisterFlagCompletionFunc("format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return docFormats, cobra.ShellCompDirectiveNoFileComp
	})
	_ = generateCmd.RegisterFlagCompletionFunc("flavor", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return mdFlavors, cobra.ShellCompDirectiveNoFileComp
	})
	_ = generateCmd.RegisterFlagCompletionFunc("pages", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return mdPageModes, cobra.ShellCompDirectiveNoFileComp
	})
	_ = generateCmd.RegisterFlagCompletionFunc("theme", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return htmlThemes, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	if !slices.Contains(docFormats, format) {
		return fmt.Errorf("unknown format %q (valid formats: %s)", format, strings.Join(docFormats, ", "))
	}

	file, _ := cmd.Flags().GetString("file")
	switch {
	case file != "" && len(args) > 0:
		return fmt.Errorf("--file and schema-id are mutually exclusive; provide one")
	case file == "" && len(args) > 0:
		return fmt.Errorf("server mode is not available yet (#918); use --file")
	case file == "":
		return fmt.Errorf("provide --file <schema.yaml> (server mode lands with #918)")
	}

	doc, err := loader.FromFile(file)
	if err != nil {
		return err
	}

	switch format {
	case "md":
		return runGenerateMD(cmd, doc)
	case "html":
		return runGenerateHTML(cmd, doc)
	case "mdx":
		return runGenerateMDX(cmd, doc)
	default:
		return doc.EncodeJSON(cmd.OutOrStdout())
	}
}

func runGenerateMD(cmd *cobra.Command, doc *docmodel.Document) error {
	flavorFlag, _ := cmd.Flags().GetString("flavor")
	if !slices.Contains(mdFlavors, flavorFlag) {
		return fmt.Errorf("unknown flavor %q (valid flavors: %s)", flavorFlag, strings.Join(mdFlavors, ", "))
	}
	pagesFlag, _ := cmd.Flags().GetString("pages")
	if !slices.Contains(mdPageModes, pagesFlag) {
		return fmt.Errorf("unknown pages mode %q (valid modes: %s)", pagesFlag, strings.Join(mdPageModes, ", "))
	}
	outDir, _ := cmd.Flags().GetString("out-dir")

	pages, err := markdown.Render(doc, markdown.Options{
		Flavor: markdown.Flavor(flavorFlag),
		Pages:  markdown.PageMode(pagesFlag),
	})
	if err != nil {
		return err
	}

	if len(pages) == 1 && outDir == "" {
		_, err := io.WriteString(cmd.OutOrStdout(), pages[0].Content)
		return err
	}
	if outDir == "" {
		return fmt.Errorf("--out-dir is required for multi-page md output")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out-dir: %w", err)
	}
	for _, p := range pages {
		path := filepath.Join(outDir, p.Name+".md")
		if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func runGenerateHTML(cmd *cobra.Command, doc *docmodel.Document) error {
	themeFlag, _ := cmd.Flags().GetString("theme")
	if !slices.Contains(htmlThemes, themeFlag) {
		return fmt.Errorf("unknown theme %q (valid themes: %s)", themeFlag, strings.Join(htmlThemes, ", "))
	}

	var userCSS string
	if cssFile, _ := cmd.Flags().GetString("css"); cssFile != "" {
		data, err := os.ReadFile(cssFile)
		if err != nil {
			return fmt.Errorf("read --css file: %w", err)
		}
		userCSS = string(data)
	}

	out, err := html.Render(doc, html.Options{Theme: html.Theme(themeFlag), CSS: userCSS})
	if err != nil {
		return err
	}

	outDir, _ := cmd.Flags().GetString("out-dir")
	if outDir == "" {
		_, err := io.WriteString(cmd.OutOrStdout(), out)
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out-dir: %w", err)
	}
	path := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func runGenerateMDX(cmd *cobra.Command, doc *docmodel.Document) error {
	outDir, _ := cmd.Flags().GetString("out-dir")
	if outDir == "" {
		return fmt.Errorf("--out-dir is required for mdx output")
	}

	pages, err := mdx.Render(doc)
	if err != nil {
		return err
	}
	for _, p := range pages {
		path := filepath.Join(outDir, p.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
