package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/tools/dump"
)

var dumpCmd = &cobra.Command{
	Use:   "dump <tenant-id>",
	Short: "Export a full tenant backup (schema + config + locks)",
	Long:  "Dump exports a tenant's schema, configuration, and field locks as a single YAML file. The output is seed-compatible and can be used to recreate the tenant elsewhere.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		var opts []dump.Option
		if mustGetBool(cmd, "no-locks") {
			opts = append(opts, dump.WithoutLocks())
		}
		if v := mustGetInt32(cmd, "version"); v > 0 {
			opts = append(opts, dump.WithConfigVersion(v))
		}

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		file, err := dump.Run(cmd.Context(), admin, args[0], opts...)
		if err != nil {
			return err
		}

		data, err := dump.Marshal(file)
		if err != nil {
			return err
		}

		outputFile := mustGetString(cmd, "output-file")
		if outputFile != "" {
			force := mustGetBool(cmd, "force")
			return writeFileExclusive(outputFile, data, force)
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

func init() {
	dumpCmd.Flags().Int32("version", 0, "config version (default: latest)")
	dumpCmd.Flags().Bool("no-locks", false, "exclude field locks")
	dumpCmd.Flags().String("output-file", "", "write to file instead of stdout")
	dumpCmd.Flags().Bool("force", false, "overwrite output file if it already exists")
}
