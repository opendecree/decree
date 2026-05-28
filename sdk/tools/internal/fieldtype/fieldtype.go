// Package fieldtype provides the canonical proto-enum-name → short-name mapping
// shared by the dump and docgen tools.
package fieldtype

// protoToShort maps proto FieldType enum names to their short YAML names.
var protoToShort = map[string]string{
	"FIELD_TYPE_INT":      "integer",
	"FIELD_TYPE_NUMBER":   "number",
	"FIELD_TYPE_STRING":   "string",
	"FIELD_TYPE_BOOL":     "bool",
	"FIELD_TYPE_TIME":     "time",
	"FIELD_TYPE_DURATION": "duration",
	"FIELD_TYPE_URL":      "url",
	"FIELD_TYPE_JSON":     "json",
}

// Name returns the short name for a proto field type enum name (e.g.
// "FIELD_TYPE_STRING" → "string"). Returns protoName unchanged if not recognized.
func Name(protoName string) string {
	if short, ok := protoToShort[protoName]; ok {
		return short
	}
	return protoName
}
