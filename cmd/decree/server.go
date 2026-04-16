package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server information and management",
	Long:  "Query server metadata, version, and enabled features.",
}

var serverInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show server version and enabled features",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		info, err := newAdminClient(conn).GetServerInfo(cmd.Context())
		if err != nil {
			return err
		}

		fmt.Printf("Version: %s\n", info.Version)
		fmt.Printf("Commit:  %s\n", info.Commit)
		fmt.Println()

		rows := tableRows([]string{"FEATURE", "ENABLED"})
		for name, enabled := range info.Features {
			val := "no"
			if enabled {
				val = "yes"
			}
			rows = append(rows, []string{name, val})
		}
		return printOutput(rows)
	},
}

func init() {
	serverCmd.AddCommand(serverInfoCmd)
}
