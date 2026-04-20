package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/tools/seed"
)

var seedCmd = &cobra.Command{
	Use:   "seed <file>",
	Short: "Seed a schema, tenant, and/or config from a YAML file",
	Long: `Seed applies a YAML file against the server. The file may contain any combination of schema, tenant, config, and locks sections; the operation dispatches based on which are present:

  schema only                  → imports the schema
  tenant only                  → creates (or reuses) the tenant
  schema + tenant              → imports schema + creates tenant
  tenant + config (+ locks)    → reuses schema, creates tenant, imports config
  schema + tenant + config     → full combined envelope (legacy form)

In config-only mode, tenant.schema names an already-imported schema. If tenant.schema_version is omitted, the latest published version is used.

The operation is idempotent: importing a schema with identical fields, or a config whose values match the latest version, is a no-op and does not create a new version.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		file, err := seed.ParseFile(data)
		if err != nil {
			return err
		}

		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		var opts []seed.Option
		if publish, _ := cmd.Flags().GetBool("auto-publish"); publish {
			opts = append(opts, seed.AutoPublish())
		}

		result, err := seed.Run(cmd.Context(), newAdminClient(conn), file, opts...)
		if err != nil {
			return err
		}

		return printOutput(tableRows(
			[]string{"RESOURCE", "ID", "CREATED", "DETAILS"},
			[]string{"schema", result.SchemaID, strconv.FormatBool(result.SchemaCreated), fmt.Sprintf("v%d", result.SchemaVersion)},
			[]string{"tenant", result.TenantID, strconv.FormatBool(result.TenantCreated), ""},
			[]string{"config", "", strconv.FormatBool(result.ConfigImported), versionOrEmpty(result.ConfigVersion)},
			[]string{"locks", "", "", fmt.Sprintf("%d applied", result.LocksApplied)},
		))
	},
}

func versionOrEmpty(v int32) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("v%d", v)
}

func init() {
	seedCmd.Flags().Bool("auto-publish", false, "auto-publish the schema version")
}
