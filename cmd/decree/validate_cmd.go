package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/tools/validate"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a config YAML against a schema YAML (offline)",
	RunE: func(cmd *cobra.Command, args []string) error {
		schemaFile := mustGetString(cmd, "schema")
		configFile := mustGetString(cmd, "config")

		schemaData, err := os.ReadFile(schemaFile)
		if err != nil {
			return fmt.Errorf("read schema: %w", err)
		}
		configData, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}

		var opts []validate.Option
		if strict := mustGetBool(cmd, "strict"); strict {
			opts = append(opts, validate.Strict())
		}

		result, err := validate.Validate(schemaData, configData, opts...)
		if err != nil {
			return err
		}

		if result.IsValid() {
			printStatus(cmd, "Valid.\n")
			return nil
		}

		fmt.Fprintf(os.Stderr, "Validation failed (%d violations):\n", len(result.Violations))
		for _, v := range result.Violations {
			fmt.Fprintf(os.Stderr, "  %s\n", v.Error())
		}
		return fmt.Errorf("validation failed with %d violation(s)", len(result.Violations))
	},
}

func init() {
	validateCmd.Flags().String("schema", "", "schema YAML file")
	_ = validateCmd.MarkFlagRequired("schema")
	validateCmd.Flags().String("config", "", "config YAML file")
	_ = validateCmd.MarkFlagRequired("config")
	validateCmd.Flags().Bool("strict", false, "reject unknown fields not in schema")
}
