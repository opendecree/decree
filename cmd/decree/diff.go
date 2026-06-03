package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/opendecree/decree/sdk/tools/diff"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between two config versions or files",
	Long: `Compare two configuration snapshots and show the differences.

Server mode (compare two versions of a tenant's config):
  decree diff <tenant-id> <version-a> <version-b>

File mode (compare two local config YAML files):
  decree diff --old config-v1.yaml --new config-v2.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		oldFile := mustGetString(cmd, "old")
		newFile := mustGetString(cmd, "new")

		var oldValues, newValues map[string]string

		if oldFile != "" {
			// File mode: compare two local YAML files.
			oldData, err := os.ReadFile(oldFile)
			if err != nil {
				return fmt.Errorf("read old file: %w", err)
			}
			newData, err := os.ReadFile(newFile)
			if err != nil {
				return fmt.Errorf("read new file: %w", err)
			}
			oldValues, err = parseConfigValues(oldData)
			if err != nil {
				return fmt.Errorf("parse old file: %w", err)
			}
			newValues, err = parseConfigValues(newData)
			if err != nil {
				return fmt.Errorf("parse new file: %w", err)
			}
		} else {
			// Server mode: compare two versions.
			if len(args) != 3 {
				return fmt.Errorf("server mode requires: <tenant-id> <version-a> <version-b>")
			}
			tenantID := args[0]
			vA, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid version-a: %w", err)
			}
			vB, err := strconv.ParseInt(args[2], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid version-b: %w", err)
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
			ctx := cmd.Context()

			va32 := int32(vA)
			vb32 := int32(vB)
			oldYAML, err := admin.ExportConfig(ctx, tenantID, &va32)
			if err != nil {
				return fmt.Errorf("export version %d: %w", vA, err)
			}
			newYAML, err := admin.ExportConfig(ctx, tenantID, &vb32)
			if err != nil {
				return fmt.Errorf("export version %d: %w", vB, err)
			}
			oldValues, err = parseConfigValues(oldYAML)
			if err != nil {
				return fmt.Errorf("parse version %d: %w", vA, err)
			}
			newValues, err = parseConfigValues(newYAML)
			if err != nil {
				return fmt.Errorf("parse version %d: %w", vB, err)
			}
		}

		result := diff.Compare(oldValues, newValues)
		if !result.HasChanges() {
			printStatus(cmd, "No changes.\n")
			return nil
		}

		fmt.Print(result.Format())
		return nil
	},
}

// parseConfigValues extracts field→value as strings from a config YAML export.
// Note: values are stringified via fmt.Sprintf("%v"), so numeric representations
// like 1.0 vs 1, or map key ordering, may produce false diffs in file mode.
func parseConfigValues(data []byte) (map[string]string, error) {
	var doc struct {
		Values map[string]struct {
			Value any `yaml:"value"`
		} `yaml:"values"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(doc.Values))
	for k, v := range doc.Values {
		m[k] = fmt.Sprintf("%v", v.Value)
	}
	return m, nil
}

func init() {
	diffCmd.Flags().String("old", "", "old config YAML file (file mode)")
	diffCmd.Flags().String("new", "", "new config YAML file (file mode)")
	diffCmd.MarkFlagsRequiredTogether("old", "new")
}
