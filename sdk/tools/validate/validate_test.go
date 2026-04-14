package validate

import (
	"testing"
)

const schemaYAML = `syntax: "v1"
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

func TestValidate_Valid(t *testing.T) {
	config := `syntax: "v1"
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
		{"integer gets string", `syntax: "v1"
values:
  count:
    value: "not-a-number"`, "count", "expected integer"},
		{"number gets bool", `syntax: "v1"
values:
  rate:
    value: true`, "rate", "expected number"},
		{"bool gets string", `syntax: "v1"
values:
  enabled:
    value: "yes"`, "enabled", "expected bool"},
		{"string gets int", `syntax: "v1"
values:
  name:
    value: 42`, "name", "expected string"},
		{"url gets int", `syntax: "v1"
values:
  endpoint:
    value: 42`, "endpoint", "expected url"},
		{"time gets int", `syntax: "v1"
values:
  start_time:
    value: 123`, "start_time", "expected time"},
		{"duration gets int", `syntax: "v1"
values:
  timeout:
    value: 123`, "timeout", "expected duration"},
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
		{"below minimum", `syntax: "v1"
values:
  rate:
    value: -1`, "less than minimum"},
		{"above maximum", `syntax: "v1"
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
	schema := `syntax: "v1"
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
		{"at exclusive min", `syntax: "v1"
values:
  val:
    value: 0`, false},
		{"at exclusive max", `syntax: "v1"
values:
  val:
    value: 10`, false},
		{"within range", `syntax: "v1"
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
		{"too short", `syntax: "v1"
values:
  name:
    value: ""`, "less than minLength"},
		{"too long", `syntax: "v1"
values:
  name:
    value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`, "exceeds maxLength"},
		{"bad pattern", `syntax: "v1"
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
	config := `syntax: "v1"
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
	schema := `syntax: "v1"
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
		name  string
		config string
		valid bool
	}{
		{"int enum match", `syntax: "v1"
values:
  level:
    value: 1`, true},
		{"int enum miss", `syntax: "v1"
values:
  level:
    value: 4`, false},
		{"float enum match", `syntax: "v1"
values:
  ratio:
    value: 1.5`, true},
		{"bool enum match", `syntax: "v1"
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
		config := `syntax: "v1"
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
		config := `syntax: "v1"
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
	config := `syntax: "v1"
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
		config := `syntax: "v1"
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
		config := `syntax: "v1"
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
}

func TestValidate_JSON(t *testing.T) {
	t.Run("structured YAML is valid JSON", func(t *testing.T) {
		config := `syntax: "v1"
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
		config := `syntax: "v1"
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
		config := `syntax: "v1"
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
		config := `syntax: "v1"
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
	config := `syntax: "v1"
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
	config := `syntax: "v1"
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

func TestValidate_InvalidPattern(t *testing.T) {
	schema := `syntax: "v1"
name: test
fields:
  x:
    type: string
    constraints:
      pattern: "[invalid"
`
	config := `syntax: "v1"
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
		Syntax: "v1",
		Name:   "test",
		Fields: map[string]FieldDef{
			"x": {Type: "integer"},
		},
	}
	config := &ConfigFile{
		Syntax: "v1",
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
	_, err := Validate([]byte("{{bad"), []byte(`syntax: "v1"
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
		{"wrong syntax", `syntax: "v2"
name: test
fields:
  a:
    type: string`},
		{"no name", `syntax: "v1"
fields:
  a:
    type: string`},
		{"no fields", `syntax: "v1"
name: test
fields: {}`},
		{"bad type", `syntax: "v1"
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
		{"wrong syntax", `syntax: "v2"
values:
  a:
    value: 1`},
		{"no values", `syntax: "v1"
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

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
