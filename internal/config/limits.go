package config

// Limits caps the number of entries in repeated request fields accepted by
// GetFields, SetFields, and Subscribe. Zero values mean "no limit". Use
// [DefaultLimits] for safe defaults.
type Limits struct {
	// MaxListLen caps the number of entries in GetFields.field_paths,
	// SetFields.updates, and Subscribe.field_paths.
	MaxListLen int
	// MaxDocBytes caps the serialized YAML document size at ImportConfig.
	MaxDocBytes int
	// MaxFieldValueBytes caps each individual field value string at ImportConfig.
	MaxFieldValueBytes int
}

// DefaultLimits returns conservative defaults.
func DefaultLimits() Limits {
	return Limits{
		MaxListLen:         1_000,
		MaxDocBytes:        5 * 1024 * 1024,
		MaxFieldValueBytes: 1 * 1024 * 1024,
	}
}
