package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/docgen"
	"github.com/opendecree/decree/sdk/tools/validate"
)

var docgenCmd = &cobra.Command{
	Use:   "docgen [schema-id]",
	Short: "Generate markdown documentation from a schema",
	Long:  "Generate markdown documentation from a schema. Provide a schema-id to fetch from the server, or --file to use a local YAML file.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file := mustGetString(cmd, "file")
		outputFile := mustGetString(cmd, "output-file")

		var schema docgen.Schema

		if file != "" {
			// Offline mode: read schema from YAML file.
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			s, err := schemaFromYAML(data)
			if err != nil {
				return err
			}
			schema = *s
		} else {
			// Online mode: fetch from server.
			if len(args) == 0 {
				return fmt.Errorf("provide a schema-id or use --file")
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
			schema = adminSchemaToDocgen(s)
		}

		var opts []docgen.Option
		if mustGetBool(cmd, "no-deprecated") {
			opts = append(opts, docgen.WithoutDeprecated())
		}
		if mustGetBool(cmd, "no-constraints") {
			opts = append(opts, docgen.WithoutConstraints())
		}
		if mustGetBool(cmd, "no-grouping") {
			opts = append(opts, docgen.WithoutGrouping())
		}

		md := docgen.Generate(schema, opts...)

		if outputFile != "" {
			force := mustGetBool(cmd, "force")
			return writeFileExclusive(outputFile, []byte(md), force)
		}
		fmt.Print(md)
		return nil
	},
}

func init() {
	docgenCmd.Flags().String("file", "", "schema YAML file (offline mode)")
	docgenCmd.Flags().Int32("version", 0, "schema version (default: latest)")
	docgenCmd.Flags().String("output-file", "", "write output to file instead of stdout")
	docgenCmd.Flags().Bool("force", false, "overwrite output file if it already exists")
	docgenCmd.Flags().Bool("no-deprecated", false, "exclude deprecated fields")
	docgenCmd.Flags().Bool("no-constraints", false, "omit constraint details")
	docgenCmd.Flags().Bool("no-grouping", false, "flat list instead of grouped by prefix")
}

// adminSchemaToDocgen converts adminclient types to docgen types.
func adminSchemaToDocgen(s *adminclient.Schema) docgen.Schema {
	ds := docgen.Schema{
		Name:        s.Name,
		Description: s.Description,
		Version:     s.Version,
		Fields:      make([]docgen.Field, len(s.Fields)),
	}
	for i, f := range s.Fields {
		ds.Fields[i] = docgen.Field{
			Path:        f.Path,
			Type:        string(f.Type),
			Description: f.Description,
			Default:     f.Default,
			Nullable:    f.Nullable,
			Deprecated:  f.Deprecated,
			RedirectTo:  f.RedirectTo,
		}
		if f.Constraints != nil {
			ds.Fields[i].Constraints = &docgen.Constraints{
				Min:          f.Constraints.Min,
				Max:          f.Constraints.Max,
				ExclusiveMin: f.Constraints.ExclusiveMin,
				ExclusiveMax: f.Constraints.ExclusiveMax,
				MinLength:    f.Constraints.MinLength,
				MaxLength:    f.Constraints.MaxLength,
				Pattern:      f.Constraints.Pattern,
				Enum:         f.Constraints.Enum,
				JSONSchema:   f.Constraints.JSONSchema,
			}
		}
	}
	return ds
}

// schemaFromYAML parses a schema YAML file into a docgen.Schema using the
// shared validate.ParseSchema helper (enforces spec_version:"v1").
func schemaFromYAML(data []byte) (*docgen.Schema, error) {
	sf, err := validate.ParseSchema(data)
	if err != nil {
		return nil, fmt.Errorf("invalid schema YAML: %w", err)
	}
	s := &docgen.Schema{
		Name:        sf.Name,
		Description: sf.Description,
		Version:     sf.Version,
	}
	for path, f := range sf.Fields {
		df := docgen.Field{
			Path:        path,
			Type:        f.Type,
			Description: f.Description,
			Default:     f.Default,
			Nullable:    f.Nullable,
			Deprecated:  f.Deprecated,
			RedirectTo:  f.RedirectTo,
			Title:       f.Title,
			Example:     f.Example,
			Tags:        f.Tags,
			Format:      f.Format,
			ReadOnly:    f.ReadOnly,
			WriteOnce:   f.WriteOnce,
			Sensitive:   f.Sensitive,
		}
		if f.Constraints != nil {
			df.Constraints = &docgen.Constraints{
				Min:          f.Constraints.Minimum,
				Max:          f.Constraints.Maximum,
				ExclusiveMin: f.Constraints.ExclusiveMinimum,
				ExclusiveMax: f.Constraints.ExclusiveMaximum,
				MinLength:    f.Constraints.MinLength,
				MaxLength:    f.Constraints.MaxLength,
				Pattern:      f.Constraints.Pattern,
				Enum:         f.Constraints.Enum,
				JSONSchema:   f.Constraints.JSONSchema,
			}
		}
		s.Fields = append(s.Fields, df)
	}
	return s, nil
}
