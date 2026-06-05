// Package validate performs offline validation of configuration YAML files
// against schema YAML definitions. No server connection is required — the
// package parses both files locally and checks type compatibility, constraint
// satisfaction, and field coverage.
package validate

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// --- YAML types (mirrors internal/schema/yaml.go and internal/config/yaml.go) ---

// SchemaFile is the parsed representation of a schema YAML file.
type SchemaFile struct {
	SpecVersion        string `yaml:"spec_version"`
	Schema             string `yaml:"$schema,omitempty"`
	ID                 string `yaml:"$id,omitempty"`
	Name               string `yaml:"name"`
	Description        string `yaml:"description,omitempty"`
	Version            int32  `yaml:"version,omitempty"`
	VersionDescription string `yaml:"version_description,omitempty"`
	// Info is accepted for forward-compatibility with richer schema formats but
	// is not used by the offline validator.
	Info   any                 `yaml:"info,omitempty"`
	Fields map[string]FieldDef `yaml:"fields"`
}

// FieldDef describes a single field in the schema YAML.
type FieldDef struct {
	Type        string          `yaml:"type"`
	Description string          `yaml:"description,omitempty"`
	Default     string          `yaml:"default,omitempty"`
	Nullable    bool            `yaml:"nullable,omitempty"`
	Deprecated  bool            `yaml:"deprecated,omitempty"`
	RedirectTo  string          `yaml:"redirect_to,omitempty"`
	Constraints *ConstraintsDef `yaml:"constraints,omitempty"`
	Title       string          `yaml:"title,omitempty"`
	Example     string          `yaml:"example,omitempty"`
	// Examples and ExternalDocs are accepted for forward-compatibility but are
	// not used by the offline validator.
	Examples     any      `yaml:"examples,omitempty"`
	ExternalDocs any      `yaml:"externalDocs,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	// Format is accepted for forward-compatibility but is not used by the
	// offline validator.
	Format    string `yaml:"format,omitempty"`
	ReadOnly  bool   `yaml:"readOnly,omitempty"`
	WriteOnce bool   `yaml:"writeOnce,omitempty"`
	Sensitive bool   `yaml:"sensitive,omitempty"`
}

// ConstraintsDef uses OAS-style naming for field constraints.
type ConstraintsDef struct {
	Minimum          *float64 `yaml:"minimum,omitempty"`
	Maximum          *float64 `yaml:"maximum,omitempty"`
	ExclusiveMinimum *float64 `yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `yaml:"exclusiveMaximum,omitempty"`
	MinLength        *int32   `yaml:"minLength,omitempty"`
	MaxLength        *int32   `yaml:"maxLength,omitempty"`
	Pattern          string   `yaml:"pattern,omitempty"`
	Enum             []string `yaml:"enum,omitempty"`
	JSONSchema       string   `yaml:"json_schema,omitempty"`
	AllowedSchemes   []string `yaml:"allowed_schemes,omitempty"`
}

// ConfigFile is the parsed representation of a config YAML file.
type ConfigFile struct {
	SpecVersion string                    `yaml:"spec_version"`
	Values      map[string]ConfigValueDef `yaml:"values"`
}

// ConfigValueDef represents a single config value in the YAML format.
type ConfigValueDef struct {
	Value       any    `yaml:"value"`
	Description string `yaml:"description,omitempty"`
}

// --- Options ---

// Option configures validation behavior.
type Option func(*options)

type options struct {
	strict bool
}

// Strict rejects fields in the config that are not defined in the schema.
func Strict() Option {
	return func(o *options) { o.strict = true }
}

// --- Result ---

// Violation describes a single validation error.
type Violation struct {
	FieldPath string
	Message   string
}

func (v Violation) Error() string {
	if v.FieldPath != "" {
		return fmt.Sprintf("%s: %s", v.FieldPath, v.Message)
	}
	return v.Message
}

// Result holds all validation violations.
type Result struct {
	Violations []Violation
}

// IsValid returns true if there are no violations.
func (r *Result) IsValid() bool {
	return len(r.Violations) == 0
}

// Error returns a multi-line summary of all violations.
func (r *Result) Error() string {
	if r.IsValid() {
		return ""
	}
	var b strings.Builder
	for i, v := range r.Violations {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(v.Error())
	}
	return b.String()
}

func (r *Result) add(fieldPath, msg string) {
	r.Violations = append(r.Violations, Violation{FieldPath: fieldPath, Message: msg})
}

// --- Public API ---

// ParseSchema parses and validates a schema YAML file.
func ParseSchema(data []byte) (*SchemaFile, error) {
	var doc SchemaFile
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if doc.SpecVersion != "v1" {
		return nil, fmt.Errorf("unsupported spec_version: %q (expected \"v1\")", doc.SpecVersion)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	if len(doc.Fields) == 0 {
		return nil, fmt.Errorf("at least one field is required")
	}
	for path, f := range doc.Fields {
		if !isValidType(f.Type) {
			return nil, fmt.Errorf("field %s: unknown type %q", path, f.Type)
		}
		if f.Default != "" {
			dv, err := parseDefaultAsTyped(f.Type, f.Default)
			if err != nil {
				return nil, fmt.Errorf("field %s: invalid default: %w", path, err)
			}
			r := &Result{}
			validateValue(r, path, dv, f)
			if !r.IsValid() {
				return nil, fmt.Errorf("field %s: default %q violates constraints: %s", path, f.Default, r.Error())
			}
		}
	}
	return &doc, nil
}

// ParseConfig parses a config YAML file.
func ParseConfig(data []byte) (*ConfigFile, error) {
	var doc ConfigFile
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if doc.SpecVersion != "v1" {
		return nil, fmt.Errorf("unsupported spec_version: %q (expected \"v1\")", doc.SpecVersion)
	}
	if len(doc.Values) == 0 {
		return nil, fmt.Errorf("at least one value is required")
	}
	return &doc, nil
}

// Validate checks a config file against a schema file.
// Both are provided as raw YAML bytes.
func Validate(schemaYAML, configYAML []byte, opts ...Option) (*Result, error) {
	schema, err := ParseSchema(schemaYAML)
	if err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	config, err := ParseConfig(configYAML)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return ValidateParsed(schema, config, opts...), nil
}

// ValidateParsed checks a parsed config against a parsed schema.
func ValidateParsed(schema *SchemaFile, config *ConfigFile, opts ...Option) *Result {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	result := &Result{}

	// Sort paths for deterministic output.
	configPaths := sortedKeys(config.Values)

	for _, path := range configPaths {
		cv := config.Values[path]
		fd, exists := schema.Fields[path]
		if !exists {
			if o.strict {
				result.add(path, "unknown field (not in schema)")
			}
			continue
		}
		validateValue(result, path, cv.Value, fd)
	}

	return result
}

// ValidateFiles is a convenience that reads files from disk.
func ValidateFiles(schemaPath, configPath string, opts ...Option) (*Result, error) {
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	return Validate(schemaData, configData, opts...)
}

// --- Validation logic ---

func validateValue(result *Result, path string, value any, fd FieldDef) {
	if value == nil {
		if !fd.Nullable {
			result.add(path, "null value for non-nullable field")
		}
		return
	}

	// typeOK tracks whether the base type check passed. If it did not, the enum
	// check is skipped to avoid a second, misleading violation for the same bad value.
	typeOK := true
	switch fd.Type {
	case "integer":
		typeOK = validateInteger(result, path, value, fd.Constraints)
	case "number":
		typeOK = validateNumber(result, path, value, fd.Constraints)
	case "string":
		typeOK = validateString(result, path, value, fd.Constraints)
	case "bool":
		typeOK = validateBool(result, path, value)
	case "time":
		typeOK = validateStringType(result, path, value, "time")
	case "duration":
		typeOK = validateDuration(result, path, value, fd.Constraints)
	case "url":
		typeOK = validateURL(result, path, value, fd.Constraints)
	case "json":
		validateJSON(result, path, value, fd.Constraints)
		// json values may be maps or slices; enum on json is not meaningful, so
		// always skip the enum check for json fields regardless of typeOK.
		return
	}

	// Enum check applies to scalar types (integer, number, string, bool, time,
	// duration, url). It is skipped when the base type validation already failed,
	// since comparing a mistyped value against the enum list yields a redundant
	// violation.
	if typeOK && fd.Constraints != nil && len(fd.Constraints.Enum) > 0 {
		validateEnum(result, path, value, fd.Constraints.Enum)
	}
}

func validateInteger(result *Result, path string, value any, c *ConstraintsDef) bool {
	var n float64
	switch v := value.(type) {
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case uint:
		n = float64(v)
	case uint64:
		n = float64(v)
	case float64:
		if v != math.Trunc(v) {
			result.add(path, fmt.Sprintf("expected integer, got %v", v))
			return false
		}
		n = v
	default:
		result.add(path, fmt.Sprintf("expected integer, got %T", value))
		return false
	}
	if c != nil {
		validateNumericConstraints(result, path, n, c)
	}
	return true
}

func validateNumber(result *Result, path string, value any, c *ConstraintsDef) bool {
	var n float64
	switch v := value.(type) {
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case uint:
		n = float64(v)
	case uint64:
		n = float64(v)
	case float64:
		n = v
	default:
		result.add(path, fmt.Sprintf("expected number, got %T", value))
		return false
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		result.add(path, "value is not a finite number")
		return false
	}
	if c != nil {
		validateNumericConstraints(result, path, n, c)
	}
	return true
}

func validateString(result *Result, path string, value any, c *ConstraintsDef) bool {
	s, ok := value.(string)
	if !ok {
		result.add(path, fmt.Sprintf("expected string, got %T", value))
		return false
	}
	if c == nil {
		return true
	}
	if c.MinLength != nil && int32(len([]rune(s))) < *c.MinLength {
		result.add(path, fmt.Sprintf("length %d is less than minLength %d", len([]rune(s)), *c.MinLength))
	}
	if c.MaxLength != nil && int32(len([]rune(s))) > *c.MaxLength {
		result.add(path, fmt.Sprintf("length %d exceeds maxLength %d", len([]rune(s)), *c.MaxLength))
	}
	if c.Pattern != "" {
		re, err := regexp.Compile(c.Pattern)
		if err != nil {
			result.add(path, fmt.Sprintf("invalid pattern %q: %v", c.Pattern, err))
		} else if !re.MatchString(s) {
			result.add(path, fmt.Sprintf("value %q does not match pattern %q", s, c.Pattern))
		}
	}
	return true
}

func validateBool(result *Result, path string, value any) bool {
	if _, ok := value.(bool); !ok {
		result.add(path, fmt.Sprintf("expected bool, got %T", value))
		return false
	}
	return true
}

func validateStringType(result *Result, path string, value any, typeName string) bool {
	s, ok := value.(string)
	if !ok {
		result.add(path, fmt.Sprintf("expected %s (string), got %T", typeName, value))
		return false
	}
	if typeName == "time" {
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			result.add(path, fmt.Sprintf("invalid time value %q: must be RFC3339 format", s))
		}
	}
	return true
}

func validateDuration(result *Result, path string, value any, _ *ConstraintsDef) bool {
	s, ok := value.(string)
	if !ok {
		result.add(path, fmt.Sprintf("expected duration (string), got %T", value))
		return false
	}
	if _, err := time.ParseDuration(s); err != nil {
		result.add(path, fmt.Sprintf("invalid duration value %q: %v", s, err))
	}
	// Numeric constraints on duration are validated server-side after parsing;
	// offline validation only checks the type and syntax.
	return true
}

func validateURL(result *Result, path string, value any, c *ConstraintsDef) bool {
	s, ok := value.(string)
	if !ok {
		result.add(path, fmt.Sprintf("expected url (string), got %T", value))
		return false
	}
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() {
		result.add(path, fmt.Sprintf("invalid absolute URL: %q", s))
		return true
	}
	schemes := []string{"http", "https"}
	if c != nil && len(c.AllowedSchemes) > 0 {
		schemes = c.AllowedSchemes
	}
	allowed := make(map[string]struct{}, len(schemes))
	for _, sc := range schemes {
		allowed[sc] = struct{}{}
	}
	if _, ok := allowed[u.Scheme]; !ok {
		result.add(path, fmt.Sprintf("URL scheme %q is not in the allowed list %v", u.Scheme, schemes))
	}
	return true
}

func validateJSON(result *Result, path string, value any, c *ConstraintsDef) {
	var jsonStr string
	switch v := value.(type) {
	case string:
		if !json.Valid([]byte(v)) {
			result.add(path, "invalid JSON string")
			return
		}
		jsonStr = v
	case map[string]any, []any:
		// Structured YAML value — valid JSON representation.
		// Marshal to JSON string for schema validation.
		b, err := json.Marshal(v)
		if err != nil {
			result.add(path, fmt.Sprintf("could not marshal JSON value: %v", err))
			return
		}
		jsonStr = string(b)
	default:
		result.add(path, fmt.Sprintf("expected JSON (string or structured), got %T", value))
		return
	}

	if c != nil && c.JSONSchema != "" {
		compiler := jsonschema.NewCompiler()
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(c.JSONSchema))
		if err != nil {
			result.add(path, fmt.Sprintf("invalid json_schema constraint: %v", err))
			return
		}
		if err := compiler.AddResource("schema.json", doc); err != nil {
			result.add(path, fmt.Sprintf("invalid json_schema constraint: %v", err))
			return
		}
		schema, err := compiler.Compile("schema.json")
		if err != nil {
			result.add(path, fmt.Sprintf("invalid json_schema constraint: %v", err))
			return
		}
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(jsonStr))
		if err != nil {
			result.add(path, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
		if err := schema.Validate(inst); err != nil {
			result.add(path, fmt.Sprintf("json schema validation failed: %v", err))
		}
	}
}

func validateEnum(result *Result, path string, value any, enum []string) {
	s := stringifyForEnum(value)
	for _, e := range enum {
		if s == e {
			return
		}
	}
	result.add(path, fmt.Sprintf("value %q is not in enum %v", s, enum))
}

func validateNumericConstraints(result *Result, path string, n float64, c *ConstraintsDef) {
	if c.Minimum != nil && n < *c.Minimum {
		result.add(path, fmt.Sprintf("value %s is less than minimum %s", formatFloat(n), formatFloat(*c.Minimum)))
	}
	if c.Maximum != nil && n > *c.Maximum {
		result.add(path, fmt.Sprintf("value %s exceeds maximum %s", formatFloat(n), formatFloat(*c.Maximum)))
	}
	if c.ExclusiveMinimum != nil && n <= *c.ExclusiveMinimum {
		result.add(path, fmt.Sprintf("value %s must be greater than %s", formatFloat(n), formatFloat(*c.ExclusiveMinimum)))
	}
	if c.ExclusiveMaximum != nil && n >= *c.ExclusiveMaximum {
		result.add(path, fmt.Sprintf("value %s must be less than %s", formatFloat(n), formatFloat(*c.ExclusiveMaximum)))
	}
}

// parseDefaultAsTyped converts the string default stored in a schema field into
// the Go value that validateValue expects for that type.
func parseDefaultAsTyped(fieldType, s string) (any, error) {
	switch fieldType {
	case "integer":
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("not a valid integer: %q", s)
		}
		return float64(n), nil
	case "number":
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("not a valid number: %q", s)
		}
		return f, nil
	case "bool":
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("not a valid bool: %q (expected true or false)", s)
		}
		return b, nil
	default:
		// string, time, duration, url, json — all stored as strings; validateValue handles format checks.
		return s, nil
	}
}

// --- Helpers ---

func isValidType(t string) bool {
	switch t {
	case "integer", "number", "string", "bool", "time", "duration", "url", "json":
		return true
	}
	return false
}

func stringifyForEnum(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return formatFloat(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
