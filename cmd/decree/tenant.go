package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenants",
}

var tenantCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new tenant on a published schema version",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		schemaID, _ := cmd.Flags().GetString("schema")
		version, _ := cmd.Flags().GetInt32("schema-version")
		if name == "" || schemaID == "" || version == 0 {
			return fmt.Errorf("--name, --schema, and --schema-version are required")
		}
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		t, err := newAdminClient(conn).CreateTenant(cmd.Context(), name, schemaID, version)
		if err != nil {
			return err
		}
		return printOutput(tableRows(
			[]string{"ID", "NAME", "SCHEMA_ID", "SCHEMA_VERSION"},
			[]string{t.ID, t.Name, t.SchemaID, strconv.Itoa(int(t.SchemaVersion))},
		))
	},
}

var tenantGetCmd = &cobra.Command{
	Use:   "get <tenant-id>",
	Short: "Show a tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		t, err := newAdminClient(conn).GetTenant(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printOutput(tableRows(
			[]string{"ID", "NAME", "SCHEMA_ID", "SCHEMA_VERSION", "CREATED_AT"},
			[]string{t.ID, t.Name, t.SchemaID, strconv.Itoa(int(t.SchemaVersion)), t.CreatedAt.Format("2006-01-02 15:04:05")},
		))
	},
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		schemaID, _ := cmd.Flags().GetString("schema")
		tenants, err := newAdminClient(conn).ListTenants(cmd.Context(), schemaID)
		if err != nil {
			return err
		}
		rows := tableRows([]string{"ID", "NAME", "SCHEMA_ID", "SCHEMA_VERSION"})
		for _, t := range tenants {
			rows = append(rows, []string{t.ID, t.Name, t.SchemaID, strconv.Itoa(int(t.SchemaVersion))})
		}
		return printOutput(rows)
	},
}

var tenantDeleteCmd = &cobra.Command{
	Use:   "delete <tenant-id>",
	Short: "Delete a tenant and all its configuration data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		if err := newAdminClient(conn).DeleteTenant(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Println("Deleted.")
		return nil
	},
}

func init() {
	tenantCreateCmd.Flags().String("name", "", "tenant name (slug)")
	tenantCreateCmd.Flags().String("schema", "", "schema ID")
	tenantCreateCmd.Flags().Int32("schema-version", 0, "published schema version")
	tenantListCmd.Flags().String("schema", "", "filter by schema ID")

	tenantCmd.AddCommand(tenantCreateCmd)
	tenantCmd.AddCommand(tenantGetCmd)
	tenantCmd.AddCommand(tenantListCmd)
	tenantCmd.AddCommand(tenantDeleteCmd)
}
