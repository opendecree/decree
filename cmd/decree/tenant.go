package main

import (
	"strconv"

	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenants",
	Long:  "Create, list, and delete tenants. Each tenant is assigned a published schema version and has its own configuration values, locks, and audit history.",
}

var tenantCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new tenant on a published schema version",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		schemaID, _ := cmd.Flags().GetString("schema")
		version, _ := cmd.Flags().GetInt32("schema-version")
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		t, err := admin.CreateTenant(cmd.Context(), name, schemaID, version)
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

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		t, err := admin.GetTenant(cmd.Context(), args[0])
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
		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		tenants, err := admin.ListTenants(cmd.Context(), schemaID)
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

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		if err := admin.DeleteTenant(cmd.Context(), args[0]); err != nil {
			return err
		}
		printStatus(cmd, "Deleted.\n")
		return nil
	},
}

func init() {
	tenantCreateCmd.Flags().String("name", "", "tenant name (slug)")
	_ = tenantCreateCmd.MarkFlagRequired("name")
	tenantCreateCmd.Flags().String("schema", "", "schema ID")
	_ = tenantCreateCmd.MarkFlagRequired("schema")
	tenantCreateCmd.Flags().Int32("schema-version", 0, "published schema version")
	_ = tenantCreateCmd.MarkFlagRequired("schema-version")
	tenantListCmd.Flags().String("schema", "", "filter by schema ID")

	tenantCmd.AddCommand(tenantCreateCmd)
	tenantCmd.AddCommand(tenantGetCmd)
	tenantCmd.AddCommand(tenantListCmd)
	tenantCmd.AddCommand(tenantDeleteCmd)
}
