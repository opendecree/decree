// Package diff computes structured diffs between two configuration snapshots.
//
// Each snapshot is a map of field paths to string values. The package is pure
// logic with no external dependencies, suitable for embedding in CLIs, servers,
// or CI pipelines.
package diff

import (
	"fmt"
	"sort"
	"strings"
)

// ChangeType categorizes a field change.
type ChangeType int

const (
	// Added means the field exists in the new snapshot but not the old.
	Added ChangeType = iota
	// Removed means the field exists in the old snapshot but not the new.
	Removed
	// Modified means the field exists in both but with different values.
	Modified
)

func (t ChangeType) String() string {
	switch t {
	case Added:
		return "added"
	case Removed:
		return "removed"
	case Modified:
		return "modified"
	default:
		return "unknown"
	}
}

// FieldChange describes a single field that differs between two snapshots.
type FieldChange struct {
	Path     string
	Type     ChangeType
	OldValue string // empty for Added
	NewValue string // empty for Removed
}

// Result holds the structured diff between two config snapshots.
type Result struct {
	Changes []FieldChange
}

// HasChanges returns true if there are any differences.
func (r *Result) HasChanges() bool {
	return len(r.Changes) > 0
}

// ByType returns only changes of the given type.
func (r *Result) ByType(t ChangeType) []FieldChange {
	var out []FieldChange
	for _, c := range r.Changes {
		if c.Type == t {
			out = append(out, c)
		}
	}
	return out
}

// Format returns a human-readable summary of the diff.
func (r *Result) Format() string {
	if !r.HasChanges() {
		return "no changes"
	}
	var b strings.Builder
	for _, c := range r.Changes {
		switch c.Type {
		case Added:
			fmt.Fprintf(&b, "+ %s = %s\n", c.Path, c.NewValue)
		case Removed:
			fmt.Fprintf(&b, "- %s = %s\n", c.Path, c.OldValue)
		case Modified:
			fmt.Fprintf(&b, "~ %s: %s → %s\n", c.Path, c.OldValue, c.NewValue)
		}
	}
	return b.String()
}

// Compare computes the diff between two config snapshots.
// Each snapshot is a map of field-path to value (as strings).
// Results are sorted by path for deterministic output.
func Compare(old, new map[string]string) *Result {
	// Collect all unique paths.
	paths := make(map[string]struct{}, len(old)+len(new))
	for p := range old {
		paths[p] = struct{}{}
	}
	for p := range new {
		paths[p] = struct{}{}
	}

	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	var changes []FieldChange
	for _, p := range sorted {
		oldVal, inOld := old[p]
		newVal, inNew := new[p]

		switch {
		case inOld && !inNew:
			changes = append(changes, FieldChange{Path: p, Type: Removed, OldValue: oldVal})
		case !inOld && inNew:
			changes = append(changes, FieldChange{Path: p, Type: Added, NewValue: newVal})
		case oldVal != newVal:
			changes = append(changes, FieldChange{Path: p, Type: Modified, OldValue: oldVal, NewValue: newVal})
		}
	}

	return &Result{Changes: changes}
}
