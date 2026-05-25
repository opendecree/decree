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

	// MaxRemoveFields caps the number of entries in UpdateSchema.remove_fields.
	MaxRemoveFields int

	// RegexPatternMaxLength caps the byte length of a regex pattern
	// accepted at schema-publish time. 0 means no limit.
	RegexPatternMaxLength int
}

// DefaultLimits returns conservative defaults: 10 000 fields, 5 MiB per
// document, 1 000 remove_fields entries per UpdateSchema request, and a
// 1 024-byte regex pattern cap. Tune via env vars at the call site (cmd/server).
func DefaultLimits() Limits {
	return Limits{
		MaxFields:             10_000,
		MaxDocBytes:           5 * 1024 * 1024,
		MaxRemoveFields:       1_000,
		RegexPatternMaxLength: 1024,
	}
}
