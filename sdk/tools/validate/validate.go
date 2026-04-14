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

	"gopkg.in/yaml.v3"
)

// --- YAML types (mirrors internal/schema/yaml.go and internal/config/yaml.go) ---

// SchemaFile is the parsed representation of a schema YAML file.
type SchemaFile struct {
	Syntax      string                `yaml:"syntax"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Fields      map[string]FieldDef   `yaml:"fields"`
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
}

// ConfigFile is the parsed representation of a config YAML file.
type ConfigFile struct {
	Syntax string                    `yaml:"syntax"`
	Values map[string]ConfigValueDef `yaml:"values"`
}

// ConfigValueDef represents a single config value in the YAML format.
type ConfigValueDef struct {
	Value       any `yaml:"value"`
	Description string      `yaml:"description,omitempty"`
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
	if doc.Syntax != "v1" {
		return nil, fmt.Errorf("unsupported syntax version: %q (expected \"v1\")", doc.Syntax)
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
	}
	return &doc, nil
}

// ParseConfig parses a config YAML file.
func ParseConfig(data []byte) (*ConfigFile, error) {
	var doc ConfigFile
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if doc.Syntax != "v1" {
		return nil, fmt.Errorf("unsupported syntax version: %q (expected \"v1\")", doc.Syntax)
	}
	if len(doc.Values) == 0 {
		return nil, fmt.Errorf("at least one value is required")
	}
	return &doc, nil
}

// Validate checks a config file against a schema file.
// Both are provided as raw YAML bytes.
func Validate(schemaYAML, configYAML []byte, opts ...Option) (*Result, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

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

	switch fd.Type {
	case "integer":
		validateInteger(result, path, value, fd.Constraints)
	case "number":
		validateNumber(result, path, value, fd.Constraints)
	case "string":
		validateString(result, path, value, fd.Constraints)
	case "bool":
		validateBool(result, path, value)
	case "time":
		validateStringType(result, path, value, "time")
	case "duration":
		validateDuration(result, path, value, fd.Constraints)
	case "url":
		validateURL(result, path, value)
	case "json":
		validateJSON(result, path, value)
	}

	// Enum check applies to any type.
	if fd.Constraints != nil && len(fd.Constraints.Enum) > 0 {
		validateEnum(result, path, value, fd.Constraints.Enum)
	}
}

func validateInteger(result *Result, path string, value any, c *ConstraintsDef) {
	var n float64
	switch v := value.(type) {
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case float64:
		if v != math.Trunc(v) {
			result.add(path, fmt.Sprintf("expected integer, got %v", v))
			return
		}
		n = v
	default:
		result.add(path, fmt.Sprintf("expected integer, got %T", value))
		return
	}
	if c != nil {
		validateNumericConstraints(result, path, n, c)
	}
}

func validateNumber(result *Result, path string, value any, c *ConstraintsDef) {
	var n float64
	switch v := value.(type) {
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case float64:
		n = v
	default:
		result.add(path, fmt.Sprintf("expected number, got %T", value))
		return
	}
	if c != nil {
		validateNumericConstraints(result, path, n, c)
	}
}

func validateString(result *Result, path string, value any, c *ConstraintsDef) {
	s, ok := value.(string)
	if !ok {
		result.add(path, fmt.Sprintf("expected string, got %T", value))
		return
	}
	if c == nil {
		return
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
}

func validateBool(result *Result, path string, value any) {
	if _, ok := value.(bool); !ok {
		result.add(path, fmt.Sprintf("expected bool, got %T", value))
	}
}

func validateStringType(result *Result, path string, value any, typeName string) {
	if _, ok := value.(string); !ok {
		result.add(path, fmt.Sprintf("expected %s (string), got %T", typeName, value))
	}
}

func validateDuration(result *Result, path string, value any, _ *ConstraintsDef) {
	if _, ok := value.(string); !ok {
		result.add(path, fmt.Sprintf("expected duration (string), got %T", value))
	}
	// Numeric constraints on duration are validated server-side after parsing;
	// offline validation only checks the type.
}

func validateURL(result *Result, path string, value any) {
	s, ok := value.(string)
	if !ok {
		result.add(path, fmt.Sprintf("expected url (string), got %T", value))
		return
	}
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() {
		result.add(path, fmt.Sprintf("invalid absolute URL: %q", s))
	}
}

func validateJSON(result *Result, path string, value any) {
	switch v := value.(type) {
	case string:
		if !json.Valid([]byte(v)) {
			result.add(path, "invalid JSON string")
		}
	case map[string]any, []any:
		// Structured YAML value — valid JSON representation.
	default:
		result.add(path, fmt.Sprintf("expected JSON (string or structured), got %T", value))
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
