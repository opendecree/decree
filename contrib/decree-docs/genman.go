package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manPageDate pins the timestamp embedded in generated man pages so the
// output is byte-deterministic across runs. cobra/doc defaults to
// time.Now(), which produces a different "Mon YYYY" header every month and
// breaks docs-up-to-date checks on the 1st of each month.
var manPageDate = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

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
			Title:   "DECREE-DOCS",
			Section: "1",
			Source:  "OpenDecree",
			Manual:  "OpenDecree CLI",
			Date:    &manPageDate,
		}

		if err := doc.GenManTree(rootCmd, header, outDir); err != nil {
			return fmt.Errorf("generate man pages: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Man pages generated in %s/\n", outDir)
		return nil
	},
}

func init() {
	// Set once here so RunE doesn't mutate global state on every invocation.
	rootCmd.DisableAutoGenTag = true

	rootCmd.AddCommand(genManCmd)
}
