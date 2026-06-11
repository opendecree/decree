package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set at build time via ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the decree-docs version",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "decree-docs %s (commit %s)\n", version, commit)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
