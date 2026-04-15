package adminclient

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Option builders + withAuth ---

func TestWithSubject(t *testing.T) {
	c := New(nil, nil, nil, WithSubject("alice"))
	if got := c.opts.subject; got != "alice" {
		t.Errorf("got subject %v, want %v", got, "alice")
	}
}

func TestWithRole(t *testing.T) {
	c := New(nil, nil, nil, WithRole("admin"))
	if got := c.opts.role; got != "admin" {
		t.Errorf("got role %v, want %v", got, "admin")
	}
}

func TestWithRole_Default(t *testing.T) {
	c := New(nil, nil, nil)
	if got := c.opts.role; got != "superadmin" {
		t.Errorf("got role %v, want %v", got, "superadmin")
	}
}

func TestWithTenantID(t *testing.T) {
	c := New(nil, nil, nil, WithTenantID("t1"))
	if got := c.opts.tenantID; got != "t1" {
		t.Errorf("got tenantID %v, want %v", got, "t1")
	}
}

func TestWithBearerToken(t *testing.T) {
	c := New(nil, nil, nil, WithBearerToken("jwt"))
	if got := c.opts.bearerToken; got != "jwt" {
		t.Errorf("got bearerToken %v, want %v", got, "jwt")
	}
}

func TestWithAuth_Metadata(t *testing.T) {
	c := New(nil, nil, nil, WithSubject("alice"), WithRole("admin"), WithTenantID("t1"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata to be present")
	}
	if got, want := md.Get("x-subject"), []string{"alice"}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got x-subject %v, want %v", got, want)
	}
	if got, want := md.Get("x-role"), []string{"admin"}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got x-role %v, want %v", got, want)
	}
	if got, want := md.Get("x-tenant-id"), []string{"t1"}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got x-tenant-id %v, want %v", got, want)
	}
}

func TestWithAuth_BearerOverrides(t *testing.T) {
	c := New(nil, nil, nil, WithSubject("alice"), WithBearerToken("jwt"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata to be present")
	}
	if got, want := md.Get("authorization"), []string{"Bearer jwt"}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got authorization %v, want %v", got, want)
	}
	if got := md.Get("x-subject"); len(got) != 0 {
		t.Errorf("expected empty x-subject, got %v", got)
	}
}

func TestWithAuth_Empty(t *testing.T) {
	c := New(nil, nil, nil, WithRole(""))
	ctx := c.withAuth(context.Background())
	_, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		t.Error("expected no outgoing metadata")
	}
}

// --- Proto conversion ---

func TestSchemaFromProto(t *testing.T) {
	now := timestamppb.Now()
	desc := "field desc"
	s := schemaFromProto(&pb.Schema{
		Id: "s1", Name: "payments", Description: "test", Version: 2,
		Checksum: "abc", Published: true, CreatedAt: now,
		Fields: []*pb.SchemaField{
			{Path: "a", Type: pb.FieldType_FIELD_TYPE_INT, Description: &desc},
			{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING, Nullable: true},
		},
	})

	if got := s.ID; got != "s1" {
		t.Errorf("got ID %v, want %v", got, "s1")
	}
	if got := s.Name; got != "payments" {
		t.Errorf("got Name %v, want %v", got, "payments")
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
	if got := s.Fields[0].Type; got != "FIELD_TYPE_INT" {
		t.Errorf("got Fields[0].Type %v, want %v", got, "FIELD_TYPE_INT")
	}
	if got := s.Fields[0].Description; got != "field desc" {
		t.Errorf("got Fields[0].Description %v, want %v", got, "field desc")
	}
	if !s.Fields[1].Nullable {
		t.Error("expected Fields[1].Nullable to be true")
	}
}

func TestSchemaFromProto_Nil(t *testing.T) {
	if got := schemaFromProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestFieldsToProto_TypeMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected pb.FieldType
	}{
		{"INT", pb.FieldType_FIELD_TYPE_INT},
		{"FIELD_TYPE_INT", pb.FieldType_FIELD_TYPE_INT},
		{"STRING", pb.FieldType_FIELD_TYPE_STRING},
		{"FIELD_TYPE_BOOL", pb.FieldType_FIELD_TYPE_BOOL},
		{"DURATION", pb.FieldType_FIELD_TYPE_DURATION},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := fieldsToProto([]Field{{Path: "x", Type: tt.input}})
			if got := result[0].Type; got != tt.expected {
				t.Errorf("got Type %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFieldsToProto_WithConstraints(t *testing.T) {
	min := 0.0
	max := 10.0
	minLen := int32(2)
	result := fieldsToProto([]Field{{
		Path: "x", Type: "INT",
		Constraints: &FieldConstraints{
			Min: &min, Max: &max, MinLength: &minLen,
			Pattern: "^[a-z]+$", Enum: []string{"a", "b"},
			JSONSchema: `{"type":"object"}`,
		},
	}})

	c := result[0].Constraints
	if c == nil {
		t.Fatal("expected non-nil Constraints")
	}
	if got := c.Min; got != &min {
		t.Errorf("got Min %v, want %v", got, &min)
	}
	if got := c.Max; got != &max {
		t.Errorf("got Max %v, want %v", got, &max)
	}
	if got := c.MinLength; got != &minLen {
		t.Errorf("got MinLength %v, want %v", got, &minLen)
	}
	if got := *c.Regex; got != "^[a-z]+$" {
		t.Errorf("got Regex %v, want %v", got, "^[a-z]+$")
	}
	if got, want := c.EnumValues, []string{"a", "b"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got EnumValues %v, want %v", got, want)
	}
	if got := *c.JsonSchema; got != `{"type":"object"}` {
		t.Errorf("got JsonSchema %v, want %v", got, `{"type":"object"}`)
	}
}

func TestFieldsToProto_OptionalFields(t *testing.T) {
	result := fieldsToProto([]Field{{
		Path: "x", Type: "STRING",
		RedirectTo:  "y",
		Default:     "hello",
		Description: "a field",
	}})

	if got := *result[0].RedirectTo; got != "y" {
		t.Errorf("got RedirectTo %v, want %v", got, "y")
	}
	if got := *result[0].DefaultValue; got != "hello" {
		t.Errorf("got DefaultValue %v, want %v", got, "hello")
	}
	if got := *result[0].Description; got != "a field" {
		t.Errorf("got Description %v, want %v", got, "a field")
	}
}

func TestTenantFromProto(t *testing.T) {
	now := timestamppb.Now()
	tenant := tenantFromProto(&pb.Tenant{
		Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 2,
		CreatedAt: now, UpdatedAt: now,
	})
	if got := tenant.Name; got != "acme" {
		t.Errorf("got Name %v, want %v", got, "acme")
	}
	if got := tenant.SchemaVersion; got != int32(2) {
		t.Errorf("got SchemaVersion %v, want %v", got, int32(2))
	}
}

func TestTenantFromProto_Nil(t *testing.T) {
	if got := tenantFromProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestVersionFromProto(t *testing.T) {
	v := versionFromProto(&pb.ConfigVersion{
		Id: "v1", TenantId: "t1", Version: 3, Description: "test",
		CreatedBy: "admin", CreatedAt: timestamppb.Now(),
	})
	if got := v.Version; got != int32(3) {
		t.Errorf("got Version %v, want %v", got, int32(3))
	}
	if got := v.CreatedBy; got != "admin" {
		t.Errorf("got CreatedBy %v, want %v", got, "admin")
	}
}

func TestVersionFromProto_Nil(t *testing.T) {
	if got := versionFromProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestAuditEntryFromProto(t *testing.T) {
	fp := "app.fee"
	old := "0.01"
	newVal := "0.02"
	ver := int32(3)
	e := auditEntryFromProto(&pb.AuditEntry{
		Id: "e1", TenantId: "t1", Actor: "admin", Action: "set_field",
		FieldPath: &fp, OldValue: &old, NewValue: &newVal,
		ConfigVersion: &ver, CreatedAt: timestamppb.Now(),
	})
	if got := e.FieldPath; got != "app.fee" {
		t.Errorf("got FieldPath %v, want %v", got, "app.fee")
	}
	if got := e.OldValue; got != "0.01" {
		t.Errorf("got OldValue %v, want %v", got, "0.01")
	}
	if got := *e.ConfigVersion; got != int32(3) {
		t.Errorf("got ConfigVersion %v, want %v", got, int32(3))
	}
}

func TestAuditEntryFromProto_Nil(t *testing.T) {
	if got := auditEntryFromProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestUsageStatsFromProto(t *testing.T) {
	lastBy := "reader"
	s := usageStatsFromProto(&pb.UsageStats{
		TenantId: "t1", FieldPath: "app.fee", ReadCount: 42,
		LastReadBy: &lastBy, LastReadAt: timestamppb.Now(),
	})
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

func TestUsageStatsFromProto_Nil(t *testing.T) {
	if got := usageStatsFromProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestPtrString(t *testing.T) {
	if got := ptrString(""); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if got := *ptrString("hello"); got != "hello" {
		t.Errorf("got %v, want %v", got, "hello")
	}
}

func TestTimeToProto(t *testing.T) {
	if got := timeToProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	now := time.Now()
	if got := timeToProto(&now); got == nil {
		t.Error("expected non-nil result")
	}
}
