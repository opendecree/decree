package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read and write configuration values",
	Long:  "Get and set typed configuration values for tenants. Supports single and batch operations, versioning, rollback, and YAML import/export.",
}

var configGetCmd = &cobra.Command{
	Use:   "get <tenant-id> <field-path>",
	Short: "Get a single config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		val, err := newConfigClient(conn).Get(cmd.Context(), args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Println(val)
		return nil
	},
}

var configGetAllCmd = &cobra.Command{
	Use:   "get-all <tenant-id>",
	Short: "Get all config values for a tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		vals, err := newConfigClient(conn).GetAll(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		rows := tableRows([]string{"FIELD", "VALUE"})
		for k, v := range vals {
			rows = append(rows, []string{k, v})
		}
		return printOutput(rows)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <tenant-id> <field-path> <value>",
	Short: "Set a single config value",
	Long: `Set a single config value.

Values are parsed according to the schema's field type:
  string    -> as-is
  integer   -> decimal integer (e.g. 42)
  number    -> float (e.g. 3.14)
  bool      -> true / false
  time      -> RFC3339 (e.g. 2006-01-02T15:04:05Z)
  duration  -> Go duration (e.g. 15s, 2h, 500ms)
  url       -> as-is
  json      -> must be valid JSON`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		if err := runConfigSet(cmd.Context(), newAdminClient(conn), newConfigClient(conn), args[0], args[1], args[2]); err != nil {
			return err
		}
		fmt.Println("Set.")
		return nil
	},
}

// runConfigSet is the testable core of `decree config set`: fetch the tenant's
// schema, coerce raw to the declared field type, and dispatch a typed write.
func runConfigSet(ctx context.Context, admin *adminclient.Client, cfg *configclient.Client, tenantID, fieldPath, raw string) error {
	types, err := tenantFieldTypes(ctx, admin, tenantID)
	if err != nil {
		return err
	}
	ft, err := lookupFieldType(types, fieldPath)
	if err != nil {
		return err
	}
	tv, err := parseTypedValue(ft, raw)
	if err != nil {
		return fmt.Errorf("field %s: %w", fieldPath, err)
	}
	return cfg.SetTyped(ctx, tenantID, fieldPath, tv)
}

var configSetManyCmd = &cobra.Command{
	Use:   "set-many <tenant-id> <key=value>...",
	Short: "Set multiple config values atomically",
	Long:  "Set multiple config values atomically. Values are parsed according to each field's schema type (see `decree config set --help`).",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenantID := args[0]
		rawValues := make(map[string]string, len(args)-1)
		for _, kv := range args[1:] {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid key=value pair: %s", kv)
			}
			rawValues[parts[0]] = parts[1]
		}
		desc, _ := cmd.Flags().GetString("description")

		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		n, err := runConfigSetMany(cmd.Context(), newAdminClient(conn), newConfigClient(conn), tenantID, rawValues, desc)
		if err != nil {
			return err
		}
		fmt.Printf("Set %d values.\n", n)
		return nil
	},
}

// runConfigSetMany is the testable core of `decree config set-many`: fetch the
// tenant's schema, coerce each raw value to its field type, and dispatch a
// typed atomic batch write. Returns the number of fields written.
func runConfigSetMany(ctx context.Context, admin *adminclient.Client, cfg *configclient.Client, tenantID string, rawValues map[string]string, description string) (int, error) {
	types, err := tenantFieldTypes(ctx, admin, tenantID)
	if err != nil {
		return 0, err
	}
	typed := make(map[string]*configclient.TypedValue, len(rawValues))
	for path, raw := range rawValues {
		ft, err := lookupFieldType(types, path)
		if err != nil {
			return 0, err
		}
		tv, err := parseTypedValue(ft, raw)
		if err != nil {
			return 0, fmt.Errorf("field %s: %w", path, err)
		}
		typed[path] = tv
	}
	if err := cfg.SetManyTyped(ctx, tenantID, typed, description); err != nil {
		return 0, err
	}
	return len(typed), nil
}

var configVersionsCmd = &cobra.Command{
	Use:   "versions <tenant-id>",
	Short: "List config versions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		versions, err := newAdminClient(conn).ListConfigVersions(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		rows := tableRows([]string{"VERSION", "DESCRIPTION", "CREATED_BY", "CREATED_AT"})
		for _, v := range versions {
			rows = append(rows, []string{
				strconv.Itoa(int(v.Version)), v.Description, v.CreatedBy,
				v.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		return printOutput(rows)
	},
}

var configRollbackCmd = &cobra.Command{
	Use:   "rollback <tenant-id> <version>",
	Short: "Rollback config to a previous version",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}
		desc, _ := cmd.Flags().GetString("description")

		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		v, err := newAdminClient(conn).RollbackConfig(cmd.Context(), args[0], int32(version), desc)
		if err != nil {
			return err
		}
		fmt.Printf("Rolled back to v%d → created v%d\n", version, v.Version)
		return nil
	},
}

var configExportCmd = &cobra.Command{
	Use:   "export <tenant-id>",
	Short: "Export config to YAML",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		var version *int32
		if v, _ := cmd.Flags().GetInt32("version"); v > 0 {
			version = &v
		}
		data, err := newAdminClient(conn).ExportConfig(cmd.Context(), args[0], version)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

var configImportCmd = &cobra.Command{
	Use:   "import <tenant-id> <file>",
	Short: "Import config from a YAML file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		desc, _ := cmd.Flags().GetString("description")
		modeStr, _ := cmd.Flags().GetString("mode")

		var mode adminclient.ImportMode
		switch modeStr {
		case "merge":
			mode = adminclient.ImportModeMerge
		case "replace":
			mode = adminclient.ImportModeReplace
		case "defaults":
			mode = adminclient.ImportModeDefaults
		default:
			mode = adminclient.ImportModeMerge
		}

		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		v, err := newAdminClient(conn).ImportConfig(cmd.Context(), args[0], data, desc, mode)
		if err != nil {
			return err
		}
		fmt.Printf("Imported → created v%d\n", v.Version)
		return nil
	},
}

func init() {
	configSetManyCmd.Flags().String("description", "", "version description")
	configRollbackCmd.Flags().String("description", "", "version description")
	configExportCmd.Flags().Int32("version", 0, "specific version (default: latest)")
	configImportCmd.Flags().String("description", "", "version description")
	configImportCmd.Flags().String("mode", "merge", "import mode: merge, replace, or defaults")
	_ = configImportCmd.RegisterFlagCompletionFunc("mode", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"merge", "replace", "defaults"}, cobra.ShellCompDirectiveNoFileComp
	})

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configGetAllCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configSetManyCmd)
	configCmd.AddCommand(configVersionsCmd)
	configCmd.AddCommand(configRollbackCmd)
	configCmd.AddCommand(configExportCmd)
	configCmd.AddCommand(configImportCmd)
}
