package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/docgen"
)

var docgenCmd = &cobra.Command{
	Use:   "docgen [schema-id]",
	Short: "Generate markdown documentation from a schema",
	Long:  "Generate markdown documentation from a schema. Provide a schema-id to fetch from the server, or --file to use a local YAML file.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		outputFile, _ := cmd.Flags().GetString("output-file")

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
			schema = adminSchemaToDocgen(s)
		}

		var opts []docgen.Option
		if noDeprecated, _ := cmd.Flags().GetBool("no-deprecated"); noDeprecated {
			opts = append(opts, docgen.WithoutDeprecated())
		}
		if noConstraints, _ := cmd.Flags().GetBool("no-constraints"); noConstraints {
			opts = append(opts, docgen.WithoutConstraints())
		}
		if noGrouping, _ := cmd.Flags().GetBool("no-grouping"); noGrouping {
			opts = append(opts, docgen.WithoutGrouping())
		}

		md := docgen.Generate(schema, opts...)

		if outputFile != "" {
			return os.WriteFile(outputFile, []byte(md), 0o644)
		}
		fmt.Print(md)
		return nil
	},
}

func init() {
	docgenCmd.Flags().String("file", "", "schema YAML file (offline mode)")
	docgenCmd.Flags().Int32("version", 0, "schema version (default: latest)")
	docgenCmd.Flags().String("output-file", "", "write output to file instead of stdout")
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
			Type:        docgen.FieldTypeName(f.Type),
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

// schemaFromYAML parses a schema YAML file into a docgen.Schema.
func schemaFromYAML(data []byte) (*docgen.Schema, error) {
	var doc struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Version     int32  `yaml:"version"`
		Fields      map[string]struct {
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
			Default     string `yaml:"default"`
			Nullable    bool   `yaml:"nullable"`
			Deprecated  bool   `yaml:"deprecated"`
			RedirectTo  string `yaml:"redirect_to"`
			Constraints *struct {
				Minimum          *float64 `yaml:"minimum"`
				Maximum          *float64 `yaml:"maximum"`
				ExclusiveMinimum *float64 `yaml:"exclusiveMinimum"`
				ExclusiveMaximum *float64 `yaml:"exclusiveMaximum"`
				MinLength        *int32   `yaml:"minLength"`
				MaxLength        *int32   `yaml:"maxLength"`
				Pattern          string   `yaml:"pattern"`
				Enum             []string `yaml:"enum"`
				JSONSchema       string   `yaml:"json_schema"`
			} `yaml:"constraints"`
		} `yaml:"fields"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid schema YAML: %w", err)
	}

	s := &docgen.Schema{
		Name:        doc.Name,
		Description: doc.Description,
		Version:     doc.Version,
	}
	for path, f := range doc.Fields {
		df := docgen.Field{
			Path:        path,
			Type:        f.Type,
			Description: f.Description,
			Default:     f.Default,
			Nullable:    f.Nullable,
			Deprecated:  f.Deprecated,
			RedirectTo:  f.RedirectTo,
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
