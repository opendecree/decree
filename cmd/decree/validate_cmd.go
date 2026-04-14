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
		schemaFile, _ := cmd.Flags().GetString("schema")
		configFile, _ := cmd.Flags().GetString("config")

		if schemaFile == "" || configFile == "" {
			return fmt.Errorf("both --schema and --config are required")
		}

		schemaData, err := os.ReadFile(schemaFile)
		if err != nil {
			return fmt.Errorf("read schema: %w", err)
		}
		configData, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}

		var opts []validate.Option
		if strict, _ := cmd.Flags().GetBool("strict"); strict {
			opts = append(opts, validate.Strict())
		}

		result, err := validate.Validate(schemaData, configData, opts...)
		if err != nil {
			return err
		}

		if result.IsValid() {
			fmt.Println("Valid.")
			return nil
		}

		fmt.Fprintf(os.Stderr, "Validation failed (%d violations):\n", len(result.Violations))
		for _, v := range result.Violations {
			fmt.Fprintf(os.Stderr, "  %s\n", v.Error())
		}
		os.Exit(1)
		return nil
	},
}

func init() {
	validateCmd.Flags().String("schema", "", "schema YAML file")
	validateCmd.Flags().String("config", "", "config YAML file")
	validateCmd.Flags().Bool("strict", false, "reject unknown fields not in schema")
}
