package validate

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

const schemaYAML = `spec_version: "v1"
name: test-schema
fields:
  rate:
    type: number
    constraints:
      minimum: 0
      maximum: 100
  name:
    type: string
    constraints:
      minLength: 1
      maxLength: 50
      pattern: "^[a-zA-Z]+$"
  enabled:
    type: bool
  count:
    type: integer
  tags:
    type: string
    nullable: true
    constraints:
      enum: [alpha, beta, stable]
  endpoint:
    type: url
  payload:
    type: json
  timeout:
    type: duration
  start_time:
    type: time
`

const validConfigYAML = `spec_version: "v1"
values:
  rate:
    value: 50.5
  name:
    value: "hello"
  enabled:
    value: true
  count:
    value: 42
  tags:
    value: "alpha"
  endpoint:
    value: "https://example.com"
  payload:
    value: {"key": "val"}
  timeout:
    value: "30s"
  start_time:
    value: "2024-01-01T00:00:00Z"
`

func TestValidate_Valid(t *testing.T) {
	config := validConfigYAML
	result, err := Validate([]byte(schemaYAML), []byte(config))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsValid() {
		t.Errorf("expected valid, got: %s", result.Error())
	}
}

func TestValidate_TypeMismatch(t *testing.T) {
	tests := []struct {
		name   string
		config string
		field  string
		msg    string
	}{
		{"integer gets string", `spec_version: "v1"
values:
  count:
    value: "not-a-number"`, "count", "expected integer"},
		{"number gets bool", `spec_version: "v1"
values:
  rate:
    value: true`, "rate", "expected number"},
		{"bool gets string", `spec_version: "v1"
values:
  enabled:
    value: "yes"`, "enabled", "expected bool"},
		{"string gets int", `spec_version: "v1"
values:
  name:
    value: 42`, "name", "expected string"},
		{"url gets int", `spec_version: "v1"
values:
  endpoint:
    value: 42`, "endpoint", "expected url"},
		{"time gets int", `spec_version: "v1"
values:
  start_time:
    value: 123`, "start_time", "expected time"},
		{"duration gets int", `spec_version: "v1"
values:
  timeout:
    value: 123`, "timeout", "expected duration"},
		{"time gets invalid string", `spec_version: "v1"
values:
  start_time:
    value: "not-a-time"`, "start_time", "invalid time value"},
		{"duration gets invalid string", `spec_version: "v1"
values:
  timeout:
    value: "not-a-duration"`, "timeout", "invalid duration value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate([]byte(schemaYAML), []byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			assertViolation(t, result, tt.field, tt.msg)
		})
	}
}

func TestValidate_NumericConstraints(t *testing.T) {
	tests := []struct {
		name   string
		config string
		msg    string
	}{
		{"below minimum", `spec_version: "v1"
values:
  rate:
    value: -1`, "less than minimum"},
		{"above maximum", `spec_version: "v1"
values:
  rate:
    value: 101`, "exceeds maximum"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate([]byte(schemaYAML), []byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			assertViolation(t, result, "rate", tt.msg)
		})
	}
}

func TestValidate_ExclusiveMinMax(t *testing.T) {
	schema := `spec_version: "v1"
name: test
fields:
  val:
    type: number
    constraints:
      exclusiveMinimum: 0
      exclusiveMaximum: 10
`
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"at exclusive min", `spec_version: "v1"
values:
  val:
    value: 0`, false},
		{"at exclusive max", `spec_version: "v1"
values:
  val:
    value: 10`, false},
		{"within range", `spec_version: "v1"
values:
  val:
    value: 5`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate([]byte(schema), []byte(tt.value))
			if err != nil {
				t.Fatal(err)
			}
			if tt.valid && !result.IsValid() {
				t.Errorf("expected valid, got: %s", result.Error())
			}
			if !tt.valid && result.IsValid() {
				t.Error("expected violation")
			}
		})
	}
}

func TestValidate_StringConstraints(t *testing.T) {
	tests := []struct {
		name   string
		config string
		msg    string
	}{
		{"too short", `spec_version: "v1"
values:
  name:
    value: ""`, "less than minLength"},
		{"too long", `spec_version: "v1"
values:
  name:
    value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`, "exceeds maxLength"},
		{"bad pattern", `spec_version: "v1"
values:
  name:
    value: "hello123"`, "does not match pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate([]byte(schemaYAML), []byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			assertViolation(t, result, "name", tt.msg)
		})
	}
}

func TestValidate_Enum(t *testing.T) {
	config := `spec_version: "v1"
values:
  tags:
    value: "invalid"
`
	result, err := Validate([]byte(schemaYAML), []byte(config))
	if err != nil {
		t.Fatal(err)
	}
	assertViolation(t, result, "tags", "not in enum")
}

func TestValidate_EnumWithNumericTypes(t *testing.T) {
	schema := `spec_version: "v1"
name: test
fields:
  level:
    type: integer
    constraints:
      enum: ["1", "2", "3"]
  ratio:
    type: number
    constraints:
      enum: ["1.5", "2.5"]
  flag:
    type: bool
    constraints:
      enum: ["true"]
`
	tests := []struct {
		name   string
		config string
		valid  bool
	}{
		{"int enum match", `spec_version: "v1"
values:
  level:
    value: 1`, true},
		{"int enum miss", `spec_version: "v1"
values:
  level:
    value: 4`, false},
		{"float enum match", `spec_version: "v1"
values:
  ratio:
    value: 1.5`, true},
		{"bool enum match", `spec_version: "v1"
values:
  flag:
    value: true`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate([]byte(schema), []byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			if tt.valid && !result.IsValid() {
				t.Errorf("expected valid, got: %s", result.Error())
			}
			if !tt.valid && result.IsValid() {
				t.Error("expected violation")
			}
		})
	}
}

func TestValidate_Nullable(t *testing.T) {
	t.Run("nullable field allows null", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  tags:
    value: null
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid, got: %s", result.Error())
		}
	})

	t.Run("non-nullable rejects null", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  count:
    value: null
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "count", "null value for non-nullable")
	})
}

func TestValidate_Strict(t *testing.T) {
	config := `spec_version: "v1"
values:
  unknown_field:
    value: "hello"
`
	t.Run("non-strict ignores unknown", func(t *testing.T) {
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid in non-strict mode, got: %s", result.Error())
		}
	})

	t.Run("strict rejects unknown", func(t *testing.T) {
		result, err := Validate([]byte(schemaYAML), []byte(config), Strict())
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "unknown_field", "unknown field")
	})
}

func TestValidate_URL(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  endpoint:
    value: "not-a-url"
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "endpoint", "invalid absolute URL")
	})

	t.Run("relative url rejected", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  endpoint:
    value: "/relative/path"
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "endpoint", "invalid absolute URL")
	})

	t.Run("mailto scheme rejected by default", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  endpoint:
    value: "mailto:user@example.com"
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "endpoint", "not in the allowed list")
	})

	t.Run("javascript scheme rejected by default", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  endpoint:
    value: "javascript:alert(1)"
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "endpoint", "not in the allowed list")
	})

	t.Run("ftp accepted with custom allowed_schemes", func(t *testing.T) {
		schema := `spec_version: "v1"
name: test
fields:
  link:
    type: url
    constraints:
      allowed_schemes: [ftp]
`
		config := `spec_version: "v1"
values:
  link:
    value: "ftp://files.example.com/pub"
`
		result, err := Validate([]byte(schema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid for ftp with allowed_schemes=[ftp], got: %s", result.Error())
		}
	})

	t.Run("http rejected with custom allowed_schemes ftp only", func(t *testing.T) {
		schema := `spec_version: "v1"
name: test
fields:
  link:
    type: url
    constraints:
      allowed_schemes: [ftp]
`
		config := `spec_version: "v1"
values:
  link:
    value: "http://example.com"
`
		result, err := Validate([]byte(schema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "link", "not in the allowed list")
	})
}

func TestValidate_NumberFiniteness(t *testing.T) {
	schema := `spec_version: "v1"
name: test
fields:
  rate:
    type: number
`
	t.Run("NaN rejected", func(t *testing.T) {
		s := &SchemaFile{
			SpecVersion: "v1",
			Name:        "test",
			Fields:      map[string]FieldDef{"rate": {Type: "number"}},
		}
		c := &ConfigFile{
			SpecVersion: "v1",
			Values:      map[string]ConfigValueDef{"rate": {Value: math.NaN()}},
		}
		result := ValidateParsed(s, c)
		assertViolation(t, result, "rate", "not a finite number")
	})

	t.Run("positive Inf rejected", func(t *testing.T) {
		s := &SchemaFile{
			SpecVersion: "v1",
			Name:        "test",
			Fields:      map[string]FieldDef{"rate": {Type: "number"}},
		}
		c := &ConfigFile{
			SpecVersion: "v1",
			Values:      map[string]ConfigValueDef{"rate": {Value: math.Inf(1)}},
		}
		result := ValidateParsed(s, c)
		assertViolation(t, result, "rate", "not a finite number")
	})

	t.Run("finite value accepted", func(t *testing.T) {
		result, err := Validate([]byte(schema), []byte(`spec_version: "v1"
values:
  rate:
    value: 3.14
`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid for finite number, got: %s", result.Error())
		}
	})
}

func TestValidate_JSONSchema(t *testing.T) {
	schema := `spec_version: "v1"
name: test
fields:
  payload:
    type: json
    constraints:
      json_schema: '{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}'
`
	t.Run("valid JSON matching schema passes", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value: '{"name":"Alice"}'
`
		result, err := Validate([]byte(schema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid, got: %s", result.Error())
		}
	})

	t.Run("JSON missing required property fails", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value: '{"age":30}'
`
		result, err := Validate([]byte(schema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "payload", "json schema validation failed")
	})

	t.Run("structured YAML matching schema passes", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value:
      name: Bob
`
		result, err := Validate([]byte(schema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid, got: %s", result.Error())
		}
	})

	t.Run("structured YAML failing schema fails", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value:
      age: 30
`
		result, err := Validate([]byte(schema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "payload", "json schema validation failed")
	})

	t.Run("invalid json_schema constraint string reports error", func(t *testing.T) {
		badSchema := `spec_version: "v1"
name: test
fields:
  payload:
    type: json
    constraints:
      json_schema: 'not valid json'
`
		config := `spec_version: "v1"
values:
  payload:
    value: '{"name":"Alice"}'
`
		result, err := Validate([]byte(badSchema), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "payload", "invalid json_schema constraint")
	})
}

func TestValidate_JSON(t *testing.T) {
	t.Run("structured YAML is valid JSON", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value:
      key: val
      nested:
        a: 1
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid, got: %s", result.Error())
		}
	})

	t.Run("JSON array is valid", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value: [1, 2, 3]
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsValid() {
			t.Errorf("expected valid, got: %s", result.Error())
		}
	})

	t.Run("invalid JSON string", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value: "{bad json"
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "payload", "invalid JSON")
	})

	t.Run("non-string non-structured rejects", func(t *testing.T) {
		config := `spec_version: "v1"
values:
  payload:
    value: 42
`
		result, err := Validate([]byte(schemaYAML), []byte(config))
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, result, "payload", "expected JSON")
	})
}

func TestValidate_IntegerRejectsFloat(t *testing.T) {
	config := `spec_version: "v1"
values:
  count:
    value: 3.14
`
	result, err := Validate([]byte(schemaYAML), []byte(config))
	if err != nil {
		t.Fatal(err)
	}
	assertViolation(t, result, "count", "expected integer")
}

func TestValidate_IntegerAcceptsWholeFloat(t *testing.T) {
	config := `spec_version: "v1"
values:
  count:
    value: 3.0
`
	result, err := Validate([]byte(schemaYAML), []byte(config))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsValid() {
		t.Errorf("expected valid (3.0 is a whole number), got: %s", result.Error())
	}
}

func TestValidate_Uint64AcceptedByInteger(t *testing.T) {
	schema := &SchemaFile{
		SpecVersion: "v1",
		Name:        "test",
		Fields: map[string]FieldDef{
			"count": {Type: "integer"},
		},
	}
	// uint64 value above int64 max (yaml.v3 decodes large integers as uint64).
	aboveInt64Max := uint64(math.MaxInt64) + 1
	for _, val := range []any{aboveInt64Max, uint(42)} {
		config := &ConfigFile{
			SpecVersion: "v1",
			Values: map[string]ConfigValueDef{
				"count": {Value: val},
			},
		}
		result := ValidateParsed(schema, config)
		if !result.IsValid() {
			t.Errorf("expected valid for %T(%v), got: %s", val, val, result.Error())
		}
	}
}

func TestValidate_Uint64AcceptedByNumber(t *testing.T) {
	schema := &SchemaFile{
		SpecVersion: "v1",
		Name:        "test",
		Fields: map[string]FieldDef{
			"rate": {Type: "number"},
		},
	}
	aboveInt64Max := uint64(math.MaxInt64) + 1
	for _, val := range []any{aboveInt64Max, uint(99)} {
		config := &ConfigFile{
			SpecVersion: "v1",
			Values: map[string]ConfigValueDef{
				"rate": {Value: val},
			},
		}
		result := ValidateParsed(schema, config)
		if !result.IsValid() {
			t.Errorf("expected valid for %T(%v), got: %s", val, val, result.Error())
		}
	}
}

func TestValidate_Uint64EnumMatch(t *testing.T) {
	schema := &SchemaFile{
		SpecVersion: "v1",
		Name:        "test",
		Fields: map[string]FieldDef{
			"level": {
				Type: "integer",
				Constraints: &ConstraintsDef{
					Enum: []string{"9223372036854775808"}, // math.MaxInt64 + 1
				},
			},
		},
	}
	aboveInt64Max := uint64(math.MaxInt64) + 1
	config := &ConfigFile{
		SpecVersion: "v1",
		Values: map[string]ConfigValueDef{
			"level": {Value: aboveInt64Max},
		},
	}
	result := ValidateParsed(schema, config)
	if !result.IsValid() {
		t.Errorf("expected enum match for uint64 above int64 max, got: %s", result.Error())
	}
}

func TestValidate_InvalidPattern(t *testing.T) {
	schema := `spec_version: "v1"
name: test
fields:
  x:
    type: string
    constraints:
      pattern: "[invalid"
`
	config := `spec_version: "v1"
values:
  x:
    value: "test"
`
	result, err := Validate([]byte(schema), []byte(config))
	if err != nil {
		t.Fatal(err)
	}
	assertViolation(t, result, "x", "invalid pattern")
}

func TestValidateParsed(t *testing.T) {
	schema := &SchemaFile{
		SpecVersion: "v1",
		Name:        "test",
		Fields: map[string]FieldDef{
			"x": {Type: "integer"},
		},
	}
	config := &ConfigFile{
		SpecVersion: "v1",
		Values: map[string]ConfigValueDef{
			"x": {Value: 42},
		},
	}

	result := ValidateParsed(schema, config)
	if !result.IsValid() {
		t.Errorf("expected valid, got: %s", result.Error())
	}
}

func TestValidate_SchemaParseError(t *testing.T) {
	_, err := Validate([]byte("{{bad"), []byte(`spec_version: "v1"
values:
  x:
    value: 1`))
	if err == nil {
		t.Error("expected error for bad schema")
	}
}

func TestValidate_ConfigParseError(t *testing.T) {
	_, err := Validate([]byte(schemaYAML), []byte("{{bad"))
	if err == nil {
		t.Error("expected error for bad config")
	}
}

func TestParseSchema_Errors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"invalid yaml", "{{bad"},
		{"wrong spec_version", `spec_version: "v2"
name: test
fields:
  a:
    type: string`},
		{"no name", `spec_version: "v1"
fields:
  a:
    type: string`},
		{"no fields", `spec_version: "v1"
name: test
fields: {}`},
		{"bad type", `spec_version: "v1"
name: test
fields:
  a:
    type: unknown`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.data))
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestParseConfig_Errors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"invalid yaml", "{{bad"},
		{"wrong spec_version", `spec_version: "v2"
values:
  a:
    value: 1`},
		{"no values", `spec_version: "v1"
values: {}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.data))
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestResult_Error(t *testing.T) {
	r := &Result{}
	if r.Error() != "" {
		t.Error("expected empty error for valid result")
	}

	r.Violations = []Violation{
		{FieldPath: "a", Message: "bad"},
		{FieldPath: "b", Message: "worse"},
	}
	errStr := r.Error()
	if errStr != "a: bad\nb: worse" {
		t.Errorf("unexpected error: %q", errStr)
	}
}

func TestViolation_Error_NoField(t *testing.T) {
	v := Violation{Message: "global error"}
	if v.Error() != "global error" {
		t.Errorf("unexpected: %q", v.Error())
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{42, "42"},
		{3.14, "3.14"},
		{0, "0"},
		{-1, "-1"},
	}
	for _, tt := range tests {
		if got := formatFloat(tt.in); got != tt.want {
			t.Errorf("formatFloat(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- Helpers ---

func assertViolation(t *testing.T, result *Result, field, msgSubstr string) {
	t.Helper()
	if result.IsValid() {
		t.Fatalf("expected violation for %s containing %q, got valid", field, msgSubstr)
	}
	for _, v := range result.Violations {
		if v.FieldPath == field && containsStr(v.Message, msgSubstr) {
			return
		}
	}
	t.Errorf("expected violation for %s containing %q, got: %s", field, msgSubstr, result.Error())
}

// --- Default value validation ---

func TestParseSchema_DefaultValidation_InvalidType(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{"integer default not a number", `spec_version: "v1"
name: test
fields:
  count:
    type: integer
    default: "notanumber"`},
		{"bool default not bool", `spec_version: "v1"
name: test
fields:
  flag:
    type: bool
    default: "yes"`},
		{"number default not a number", `spec_version: "v1"
name: test
fields:
  ratio:
    type: number
    default: "abc"`},
		{"time default invalid format", `spec_version: "v1"
name: test
fields:
  ts:
    type: time
    default: "not-a-time"`},
		{"url default not absolute", `spec_version: "v1"
name: test
fields:
  endpoint:
    type: url
    default: "relative/path"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.schema))
			if err == nil {
				t.Error("expected error for invalid default, got nil")
			}
		})
	}
}

func TestParseSchema_DefaultValidation_ConstraintViolation(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{"integer default below minimum", `spec_version: "v1"
name: test
fields:
  count:
    type: integer
    default: "3"
    constraints:
      minimum: 10`},
		{"string default below minLength", `spec_version: "v1"
name: test
fields:
  code:
    type: string
    default: "ab"
    constraints:
      minLength: 5`},
		{"default not in enum", `spec_version: "v1"
name: test
fields:
  color:
    type: string
    default: "purple"
    constraints:
      enum: [red, green, blue]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.schema))
			if err == nil {
				t.Error("expected error for default violating constraints, got nil")
			}
		})
	}
}

func TestParseSchema_DefaultValidation_Valid(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{"integer default valid", `spec_version: "v1"
name: test
fields:
  count:
    type: integer
    default: "42"`},
		{"bool default true", `spec_version: "v1"
name: test
fields:
  flag:
    type: bool
    default: "true"`},
		{"string default within constraints", `spec_version: "v1"
name: test
fields:
  code:
    type: string
    default: "hello"
    constraints:
      minLength: 3
      maxLength: 10`},
		{"string default in enum", `spec_version: "v1"
name: test
fields:
  color:
    type: string
    default: "red"
    constraints:
      enum: [red, green, blue]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.schema))
			if err != nil {
				t.Errorf("expected no error for valid default, got: %v", err)
			}
		})
	}
}

// --- Large-integer precision ---

func TestValidate_Uint64NearMaxUint64(t *testing.T) {
	// uint64(math.MaxUint64) converted to float64 loses precision.
	// validateInteger must still accept it without a spurious "not integer" error.
	schema := &SchemaFile{
		SpecVersion: "v1",
		Name:        "test",
		Fields:      map[string]FieldDef{"n": {Type: "integer"}},
	}
	config := &ConfigFile{
		SpecVersion: "v1",
		Values:      map[string]ConfigValueDef{"n": {Value: uint64(math.MaxUint64)}},
	}
	result := ValidateParsed(schema, config)
	if !result.IsValid() {
		t.Errorf("expected valid for uint64(MaxUint64), got: %s", result.Error())
	}
}

// --- Double-violation: malformed duration + enum ---

func TestValidate_Duration_MalformedAndNotInEnum(t *testing.T) {
	// A duration field with an enum constraint that also has a malformed value
	// should produce two independent violations: one for invalid syntax, one for
	// the enum mismatch. This exercises the fact that validateEnum runs after
	// type-specific validation regardless of whether the type check passed.
	schema := &SchemaFile{
		SpecVersion: "v1",
		Name:        "test",
		Fields: map[string]FieldDef{
			"timeout": {
				Type:        "duration",
				Constraints: &ConstraintsDef{Enum: []string{"1s", "5s", "30s"}},
			},
		},
	}
	config := &ConfigFile{
		SpecVersion: "v1",
		Values:      map[string]ConfigValueDef{"timeout": {Value: "bad-duration"}},
	}
	result := ValidateParsed(schema, config)
	if result.IsValid() {
		t.Fatal("expected violations, got valid")
	}
	if len(result.Violations) < 2 {
		t.Errorf("expected at least 2 violations (syntax + enum), got %d: %s", len(result.Violations), result.Error())
	}
}

// --- ValidateFiles ---

func TestValidateFiles_HappyPath(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	configPath := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(schemaPath, []byte(schemaYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := ValidateFiles(schemaPath, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid() {
		t.Errorf("expected valid, got: %s", result.Error())
	}
}

func TestValidateFiles_MissingSchemaFile(t *testing.T) {
	_, err := ValidateFiles("/nonexistent/schema.yaml", "/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing schema file, got nil")
	}
}

func TestValidateFiles_MissingConfigFile(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte(schemaYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ValidateFiles(schemaPath, "/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
