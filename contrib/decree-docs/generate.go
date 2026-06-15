package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/contrib/decree-docs/loader"
)

// docFormats lists the formats generate can emit. The md, mdx, and html
// backends land in upcoming releases (#915-#917).
var docFormats = []string{"json"}

var generateCmd = &cobra.Command{
	Use:   "generate [schema-id]",
	Short: "Generate documentation from a decree schema",
	Long: `Generate documentation from a decree schema.

Provide --file to load a schema YAML file. Fetching a schema-id from a
running server arrives together with the server connection flags in an
upcoming release (#918); --file and schema-id are mutually exclusive.

The json format emits the complete documentation model: a stable JSON
document, versioned by its docModelVersion root marker, that third-party
renderers can build on.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().String("file", "", "schema YAML file (offline mode)")
	generateCmd.Flags().String("format", "json", "output format: "+strings.Join(docFormats, ", "))
	_ = generateCmd.RegisterFlagCompletionFunc("format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return docFormats, cobra.ShellCompDirectiveNoFileComp
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
	// Only json today; this switch grows with the md/mdx/html backends.
	return doc.EncodeJSON(cmd.OutOrStdout())
}
