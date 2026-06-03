package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set at build time via ldflags.
var (
	cliVersion = "dev"
	cliCommit  = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("decree %s (commit %s)\n", cliVersion, cliCommit)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
