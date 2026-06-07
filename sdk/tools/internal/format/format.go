// Package format holds small formatting helpers shared across the tools module.
package format

import "fmt"

// Float formats a float64 for display, omitting the decimal point when the
// value is a whole number (e.g. 1.0 -> "1", 1.5 -> "1.5").
func Float(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}
