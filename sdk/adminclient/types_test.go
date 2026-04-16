package adminclient

import (
	"testing"
	"time"
)

// Types are plain structs — these tests verify field assignment
// and zero-value semantics that callers rely on.

func TestSchema_Fields(t *testing.T) {
	now := time.Now()
	parent := int32(1)
	s := Schema{
		ID:                 "s1",
		Name:               "payments",
		Description:        "Payment config",
		Version:            2,
		ParentVersion:      &parent,
		VersionDescription: "added fee",
		Checksum:           "abc123",
		Published:          true,
		Fields: []Field{
			{Path: "fee", Type: "NUMBER"},
			{Path: "name", Type: "STRING", Nullable: true},
		},
		CreatedAt: now,
		Info: &SchemaInfo{
			Title:  "Payment Config",
			Author: "payments-team",
			Contact: &SchemaContact{
				Name:  "Payments Team",
				Email: "pay@example.com",
				URL:   "https://wiki.example.com",
			},
			Labels: map[string]string{"team": "payments"},
		},
	}

	if got := s.ID; got != "s1" {
		t.Errorf("got ID %v, want %v", got, "s1")
	}
	if got := s.Version; got != int32(2) {
		t.Errorf("got Version %v, want %v", got, int32(2))
	}
	if !s.Published {
		t.Error("expected Published to be true")
	}
	if len(s.Fields) != 2 {
		t.Errorf("got len %d, want %d", len(s.Fields), 2)
	}
	if s.ParentVersion == nil || *s.ParentVersion != int32(1) {
		t.Errorf("got ParentVersion %v, want 1", s.ParentVersion)
	}
	if s.Info == nil {
		t.Fatal("expected non-nil Info")
	}
	if got := s.Info.Title; got != "Payment Config" {
		t.Errorf("got Info.Title %v, want %v", got, "Payment Config")
	}
	if s.Info.Contact == nil {
		t.Fatal("expected non-nil Info.Contact")
	}
	if got := s.Info.Contact.URL; got != "https://wiki.example.com" {
		t.Errorf("got Info.Contact.URL %v, want %v", got, "https://wiki.example.com")
	}
	if got := s.Info.Labels["team"]; got != "payments" {
		t.Errorf("got Info.Labels[team] %v, want %v", got, "payments")
	}
}

func TestField_AllMetadata(t *testing.T) {
	f := Field{
		Path:        "payments.fee",
		Type:        "NUMBER",
		Nullable:    true,
		Deprecated:  true,
		RedirectTo:  "payments.new_fee",
		Default:     "0.01",
		Description: "Transaction fee",
		Title:       "Fee Rate",
		Example:     "0.025",
		Format:      "percentage",
		Tags:        []string{"billing", "critical"},
		ReadOnly:    true,
		WriteOnce:   true,
		Sensitive:   true,
		Examples: map[string]FieldExample{
			"low":  {Value: "0.01", Summary: "Low rate"},
			"high": {Value: "0.99", Summary: "High rate"},
		},
		ExternalDocs: &ExternalDocs{
			Description: "Fee guide",
			URL:         "https://docs.example.com/fees",
		},
		Constraints: &FieldConstraints{
			Pattern: "^[0-9.]+$",
			Enum:    []string{"0.01", "0.05", "0.10"},
		},
	}

	if got := f.Path; got != "payments.fee" {
		t.Errorf("got Path %v, want %v", got, "payments.fee")
	}
	if got := f.Title; got != "Fee Rate" {
		t.Errorf("got Title %v, want %v", got, "Fee Rate")
	}
	if got := f.Example; got != "0.025" {
		t.Errorf("got Example %v, want %v", got, "0.025")
	}
	if got := f.Format; got != "percentage" {
		t.Errorf("got Format %v, want %v", got, "percentage")
	}
	if !f.ReadOnly {
		t.Error("expected ReadOnly to be true")
	}
	if !f.WriteOnce {
		t.Error("expected WriteOnce to be true")
	}
	if !f.Sensitive {
		t.Error("expected Sensitive to be true")
	}
	if !f.Nullable {
		t.Error("expected Nullable to be true")
	}
	if !f.Deprecated {
		t.Error("expected Deprecated to be true")
	}
	if got := f.RedirectTo; got != "payments.new_fee" {
		t.Errorf("got RedirectTo %v, want %v", got, "payments.new_fee")
	}
	if got := f.Default; got != "0.01" {
		t.Errorf("got Default %v, want %v", got, "0.01")
	}
	if got := f.Description; got != "Transaction fee" {
		t.Errorf("got Description %v, want %v", got, "Transaction fee")
	}

	if len(f.Examples) != 2 {
		t.Fatalf("got len %d, want %d", len(f.Examples), 2)
	}
	if got := f.Examples["low"].Value; got != "0.01" {
		t.Errorf("got Examples[low].Value %v, want %v", got, "0.01")
	}

	if f.ExternalDocs == nil {
		t.Fatal("expected non-nil ExternalDocs")
	}
	if got := f.ExternalDocs.URL; got != "https://docs.example.com/fees" {
		t.Errorf("got ExternalDocs.URL %v, want %v", got, "https://docs.example.com/fees")
	}

	if f.Constraints == nil {
		t.Fatal("expected non-nil Constraints")
	}
	if got := f.Constraints.Pattern; got != "^[0-9.]+$" {
		t.Errorf("got Constraints.Pattern %v, want %v", got, "^[0-9.]+$")
	}
	if len(f.Constraints.Enum) != 3 {
		t.Errorf("got Constraints.Enum len %d, want %d", len(f.Constraints.Enum), 3)
	}
}

func TestFieldConstraints_AllFields(t *testing.T) {
	min := 0.0
	max := 100.0
	exMin := 0.0
	exMax := 100.0
	minLen := int32(1)
	maxLen := int32(255)

	c := FieldConstraints{
		Min:          &min,
		Max:          &max,
		ExclusiveMin: &exMin,
		ExclusiveMax: &exMax,
		MinLength:    &minLen,
		MaxLength:    &maxLen,
		Pattern:      "^[a-z]+$",
		Enum:         []string{"a", "b"},
		JSONSchema:   `{"type":"object"}`,
	}

	if c.Min == nil || *c.Min != 0.0 {
		t.Errorf("got Min %v, want 0.0", c.Min)
	}
	if c.Max == nil || *c.Max != 100.0 {
		t.Errorf("got Max %v, want 100.0", c.Max)
	}
	if c.MinLength == nil || *c.MinLength != int32(1) {
		t.Errorf("got MinLength %v, want 1", c.MinLength)
	}
	if c.MaxLength == nil || *c.MaxLength != int32(255) {
		t.Errorf("got MaxLength %v, want 255", c.MaxLength)
	}
	if got := c.JSONSchema; got != `{"type":"object"}` {
		t.Errorf("got JSONSchema %v, want %v", got, `{"type":"object"}`)
	}
}

func TestVersion_Fields(t *testing.T) {
	now := time.Now()
	v := Version{
		ID:          "v1",
		TenantID:    "t1",
		Version:     3,
		Description: "test version",
		CreatedBy:   "admin",
		CreatedAt:   now,
	}

	if got := v.Version; got != int32(3) {
		t.Errorf("got Version %v, want %v", got, int32(3))
	}
	if got := v.CreatedBy; got != "admin" {
		t.Errorf("got CreatedBy %v, want %v", got, "admin")
	}
	if got := v.Description; got != "test version" {
		t.Errorf("got Description %v, want %v", got, "test version")
	}
}

func TestAuditEntry_Fields(t *testing.T) {
	ver := int32(3)
	e := AuditEntry{
		ID:            "e1",
		TenantID:      "t1",
		Actor:         "admin",
		Action:        "set_field",
		FieldPath:     "app.fee",
		OldValue:      "0.01",
		NewValue:      "0.02",
		ConfigVersion: &ver,
		CreatedAt:     time.Now(),
	}

	if got := e.FieldPath; got != "app.fee" {
		t.Errorf("got FieldPath %v, want %v", got, "app.fee")
	}
	if got := e.OldValue; got != "0.01" {
		t.Errorf("got OldValue %v, want %v", got, "0.01")
	}
	if e.ConfigVersion == nil || *e.ConfigVersion != int32(3) {
		t.Errorf("got ConfigVersion %v, want 3", e.ConfigVersion)
	}
}

func TestUsageStats_Fields(t *testing.T) {
	now := time.Now()
	s := UsageStats{
		TenantID:   "t1",
		FieldPath:  "app.fee",
		ReadCount:  42,
		LastReadBy: "reader",
		LastReadAt: &now,
	}

	if got := s.ReadCount; got != int64(42) {
		t.Errorf("got ReadCount %v, want %v", got, int64(42))
	}
	if got := s.LastReadBy; got != "reader" {
		t.Errorf("got LastReadBy %v, want %v", got, "reader")
	}
	if s.LastReadAt == nil {
		t.Error("expected non-nil LastReadAt")
	}
}

func TestUsageStats_NilLastReadAt(t *testing.T) {
	s := UsageStats{ReadCount: 0}
	if s.LastReadAt != nil {
		t.Errorf("expected nil LastReadAt, got %v", s.LastReadAt)
	}
	if got := s.LastReadBy; got != "" {
		t.Errorf("expected empty LastReadBy, got %v", got)
	}
}

func TestImportMode_Constants(t *testing.T) {
	if ImportModeMerge != 1 {
		t.Errorf("got ImportModeMerge %v, want 1", ImportModeMerge)
	}
	if ImportModeReplace != 2 {
		t.Errorf("got ImportModeReplace %v, want 2", ImportModeReplace)
	}
	if ImportModeDefaults != 3 {
		t.Errorf("got ImportModeDefaults %v, want 3", ImportModeDefaults)
	}
}
