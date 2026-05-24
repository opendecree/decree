package config

// Limits caps the number of entries in repeated request fields accepted by
// GetFields, SetFields, and Subscribe. Zero values mean "no limit". Use
// [DefaultLimits] for safe defaults.
type Limits struct {
	// MaxListLen caps the number of entries in GetFields.field_paths,
	// SetFields.updates, and Subscribe.field_paths.
	MaxListLen int
}

// DefaultLimits returns a conservative default: 1 000 entries per list.
func DefaultLimits() Limits {
	return Limits{
		MaxListLen: 1_000,
	}
}
