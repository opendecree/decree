package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/adminclient"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Manage configuration schemas",
	Long:  "Create, list, publish, import/export, and delete configuration schemas. Schemas define the allowed fields, types, and constraints for tenant configurations.",
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
		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}

		version := mustGetInt32(cmd, "version")
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
			rows = append(rows, []string{f.Path, string(f.Type), strconv.FormatBool(f.Nullable), strconv.FormatBool(f.Deprecated), f.Description})
		}
		printStatus(cmd, "Schema: %s (%s) v%d [published=%v]\n\n", s.Name, s.ID, s.Version, s.Published)
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

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		schemas, err := admin.ListSchemas(cmd.Context())
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
		parsedVersion, err := strconv.ParseInt(args[1], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		s, err := admin.PublishSchema(cmd.Context(), args[0], int32(parsedVersion))
		if err != nil {
			return err
		}
		printStatus(cmd, "Published %s v%d\n", s.Name, s.Version)
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

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		if err := admin.DeleteSchema(cmd.Context(), args[0]); err != nil {
			return err
		}
		printStatus(cmd, "Deleted.\n")
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
		if v := mustGetInt32(cmd, "version"); v > 0 {
			version = &v
		}
		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		data, err := admin.ExportSchema(cmd.Context(), args[0], version)
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

		publish := mustGetBool(cmd, "publish")
		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		var importOpts []adminclient.ImportSchemaOption
		if publish {
			importOpts = append(importOpts, adminclient.WithAutoPublish())
		}
		s, err := admin.ImportSchema(cmd.Context(), data, importOpts...)
		if err != nil {
			return err
		}
		if s.Published {
			printStatus(cmd, "Imported and published %s v%d\n", s.Name, s.Version)
		} else {
			printStatus(cmd, "Imported %s v%d (draft)\n", s.Name, s.Version)
		}
		return nil
	},
}

func init() {
	schemaGetCmd.Flags().Int32("version", 0, "specific version (default: latest)")
	schemaExportCmd.Flags().Int32("version", 0, "specific version (default: latest)")
	schemaImportCmd.Flags().Bool("publish", false, "auto-publish the imported version")

	schemaCmd.AddCommand(schemaGetCmd)
	schemaCmd.AddCommand(schemaListCmd)
	schemaCmd.AddCommand(schemaPublishCmd)
	schemaCmd.AddCommand(schemaDeleteCmd)
	schemaCmd.AddCommand(schemaExportCmd)
	schemaCmd.AddCommand(schemaImportCmd)
}
