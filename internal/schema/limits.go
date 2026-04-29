package schema

// Limits caps the size and complexity of schema documents accepted by
// CreateSchema and ImportSchema. Zero values mean "no limit" for that
// dimension. Use [DefaultLimits] for safe defaults.
//
// Limits guard against pathological inputs that would otherwise hang or
// OOM the server (cyclic $ref, exponential allOf/anyOf, million-element
// enum). They are tracked in opendecree/decree#217 (security review).
type Limits struct {
	// MaxFields caps the number of SchemaField entries per schema version.
	MaxFields int

	// MaxDocBytes caps the serialized YAML document size at ImportSchema.
	MaxDocBytes int
}

// DefaultLimits returns conservative defaults: 10 000 fields and 5 MiB
// per document. Tune via env vars at the call site (cmd/server).
func DefaultLimits() Limits {
	return Limits{
		MaxFields:   10_000,
		MaxDocBytes: 5 * 1024 * 1024,
	}
}
