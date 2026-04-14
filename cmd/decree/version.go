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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ccs %s (commit %s)\n", cliVersion, cliCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
