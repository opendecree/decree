// Config Validation demonstrates offline validation of config values
// against a schema — no server connection required. This is ideal for
// CI pipelines that gate deployments on config correctness.
//
// Run:
//
//	go run .
package main

import (
	"fmt"
	"log"

	"github.com/opendecree/decree/sdk/tools/validate"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Validate a correct config.
	fmt.Println("=== Valid config ===")
	result, err := validate.ValidateFiles("decree.schema.yaml", "valid.decree.config.yaml")
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if result.IsValid() {
		fmt.Println("  PASS: no violations")
	}

	// Validate a config with errors.
	fmt.Println()
	fmt.Println("=== Invalid config ===")
	result, err = validate.ValidateFiles("decree.schema.yaml", "invalid.decree.config.yaml", validate.Strict())
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if !result.IsValid() {
		fmt.Println("  FAIL:")
		for _, v := range result.Violations {
			fmt.Printf("    - %s: %s\n", v.FieldPath, v.Message)
		}
	}

	return nil
}
