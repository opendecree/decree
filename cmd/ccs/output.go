package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// printOutput writes data in the format selected by --output flag.
func printOutput(data any) error {
	switch flagOutput {
	case "json":
		return printJSON(os.Stdout, data)
	case "yaml":
		return printYAML(os.Stdout, data)
	default:
		return printTable(os.Stdout, data)
	}
}

func printJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func printYAML(w io.Writer, data any) error {
	return yaml.NewEncoder(w).Encode(data)
}

// printTable formats data as an aligned table.
// data must be a [][]string where the first row is headers.
func printTable(w io.Writer, data any) error {
	rows, ok := data.([][]string)
	if !ok {
		// Fall back to JSON for non-table data.
		return printJSON(w, data)
	}
	if len(rows) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, row := range rows {
		line := strings.Join(row, "\t")
		if i == 0 {
			_, _ = fmt.Fprintln(tw, line)
			// Separator line.
			sep := make([]string, len(row))
			for j, col := range row {
				sep[j] = strings.Repeat("-", len(col))
			}
			_, _ = fmt.Fprintln(tw, strings.Join(sep, "\t"))
		} else {
			_, _ = fmt.Fprintln(tw, line)
		}
	}
	return tw.Flush()
}

// tableRows is a helper to build table data with headers.
func tableRows(headers []string, rows ...[]string) [][]string {
	result := make([][]string, 0, 1+len(rows))
	result = append(result, headers)
	result = append(result, rows...)
	return result
}
