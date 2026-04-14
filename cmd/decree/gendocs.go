package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var genDocsCmd = &cobra.Command{
	Use:    "gen-docs [output-dir]",
	Short:  "Generate CLI reference documentation in markdown",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outDir := "docs/cli"
		if len(args) > 0 {
			outDir = args[0]
		}

		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}

		prepender := func(filename string) string {
			name := filepath.Base(filename)
			name = strings.TrimSuffix(name, filepath.Ext(name))
			title := strings.ReplaceAll(name, "_", " ")
			return fmt.Sprintf("---\ntitle: %s\n---\n\n", title)
		}

		linkHandler := func(name string) string {
			return strings.ToLower(name)
		}

		rootCmd.DisableAutoGenTag = true
		if err := doc.GenMarkdownTreeCustom(rootCmd, outDir, prepender, linkHandler); err != nil {
			return fmt.Errorf("generate docs: %w", err)
		}

		fmt.Printf("CLI docs generated in %s/\n", outDir)
		return nil
	},
}

var genManCmd = &cobra.Command{
	Use:    "gen-man [output-dir]",
	Short:  "Generate man pages",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outDir := "docs/man"
		if len(args) > 0 {
			outDir = args[0]
		}

		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}

		header := &doc.GenManHeader{
			Title:   "DECREE",
			Section: "1",
			Source:  "OpenDecree",
			Manual:  "OpenDecree CLI",
		}

		rootCmd.DisableAutoGenTag = true
		if err := doc.GenManTree(rootCmd, header, outDir); err != nil {
			return fmt.Errorf("generate man pages: %w", err)
		}

		fmt.Printf("Man pages generated in %s/\n", outDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(genDocsCmd)
	rootCmd.AddCommand(genManCmd)
}
