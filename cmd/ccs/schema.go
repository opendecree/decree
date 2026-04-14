package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/zeevdr/central-config-service/sdk/adminclient"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Manage configuration schemas",
}

var schemaCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new schema from a YAML file",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			return fmt.Errorf("--file is required")
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		s, err := newAdminClient(conn).ImportSchema(cmd.Context(), data)
		if err != nil {
			return err
		}
		return printOutput(tableRows(
			[]string{"ID", "NAME", "VERSION", "PUBLISHED"},
			[]string{s.ID, s.Name, strconv.Itoa(int(s.Version)), strconv.FormatBool(s.Published)},
		))
	},
}

var schemaGetCmd = &cobra.Command{
	Use:   "get <schema-id>",
	Short: "Show a schema",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		admin := newAdminClient(conn)

		version, _ := cmd.Flags().GetInt32("version")
		var s *adminclient.Schema
		if version > 0 {
			s, err = admin.GetSchemaVersion(cmd.Context(), args[0], version)
		} else {
			s, err = admin.GetSchema(cmd.Context(), args[0])
		}
		if err != nil {
			return err
		}

		rows := tableRows([]string{"PATH", "TYPE", "NULLABLE", "DEPRECATED", "DESCRIPTION"})
		for _, f := range s.Fields {
			rows = append(rows, []string{f.Path, f.Type, strconv.FormatBool(f.Nullable), strconv.FormatBool(f.Deprecated), f.Description})
		}
		fmt.Printf("Schema: %s (%s) v%d [published=%v]\n\n", s.Name, s.ID, s.Version, s.Published)
		return printOutput(rows)
	},
}

var schemaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all schemas",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		schemas, err := newAdminClient(conn).ListSchemas(cmd.Context())
		if err != nil {
			return err
		}
		rows := tableRows([]string{"ID", "NAME", "VERSION", "PUBLISHED"})
		for _, s := range schemas {
			rows = append(rows, []string{s.ID, s.Name, strconv.Itoa(int(s.Version)), strconv.FormatBool(s.Published)})
		}
		return printOutput(rows)
	},
}

var schemaPublishCmd = &cobra.Command{
	Use:   "publish <schema-id> <version>",
	Short: "Publish a schema version (makes it immutable and assignable to tenants)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		s, err := newAdminClient(conn).PublishSchema(cmd.Context(), args[0], int32(version))
		if err != nil {
			return err
		}
		fmt.Printf("Published %s v%d\n", s.Name, s.Version)
		return nil
	},
}

var schemaDeleteCmd = &cobra.Command{
	Use:   "delete <schema-id>",
	Short: "Delete a schema and all its versions (cascades to tenants)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		if err := newAdminClient(conn).DeleteSchema(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Println("Deleted.")
		return nil
	},
}

var schemaExportCmd = &cobra.Command{
	Use:   "export <schema-id>",
	Short: "Export a schema to YAML",
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
		data, err := newAdminClient(conn).ExportSchema(cmd.Context(), args[0], version)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

var schemaImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a schema from a YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		s, err := newAdminClient(conn).ImportSchema(cmd.Context(), data)
		if err != nil {
			return err
		}
		fmt.Printf("Imported %s v%d\n", s.Name, s.Version)
		return nil
	},
}

func init() {
	schemaCreateCmd.Flags().StringP("file", "f", "", "YAML file with schema definition")
	schemaGetCmd.Flags().Int32("version", 0, "specific version (default: latest)")
	schemaExportCmd.Flags().Int32("version", 0, "specific version (default: latest)")

	schemaCmd.AddCommand(schemaCreateCmd)
	schemaCmd.AddCommand(schemaGetCmd)
	schemaCmd.AddCommand(schemaListCmd)
	schemaCmd.AddCommand(schemaPublishCmd)
	schemaCmd.AddCommand(schemaDeleteCmd)
	schemaCmd.AddCommand(schemaExportCmd)
	schemaCmd.AddCommand(schemaImportCmd)
}
