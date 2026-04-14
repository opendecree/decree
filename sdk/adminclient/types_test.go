package adminclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Option builders + withAuth ---

func TestWithSubject(t *testing.T) {
	c := New(nil, nil, nil, WithSubject("alice"))
	assert.Equal(t, "alice", c.opts.subject)
}

func TestWithRole(t *testing.T) {
	c := New(nil, nil, nil, WithRole("admin"))
	assert.Equal(t, "admin", c.opts.role)
}

func TestWithRole_Default(t *testing.T) {
	c := New(nil, nil, nil)
	assert.Equal(t, "superadmin", c.opts.role)
}

func TestWithTenantID(t *testing.T) {
	c := New(nil, nil, nil, WithTenantID("t1"))
	assert.Equal(t, "t1", c.opts.tenantID)
}

func TestWithBearerToken(t *testing.T) {
	c := New(nil, nil, nil, WithBearerToken("jwt"))
	assert.Equal(t, "jwt", c.opts.bearerToken)
}

func TestWithAuth_Metadata(t *testing.T) {
	c := New(nil, nil, nil, WithSubject("alice"), WithRole("admin"), WithTenantID("t1"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"alice"}, md.Get("x-subject"))
	assert.Equal(t, []string{"admin"}, md.Get("x-role"))
	assert.Equal(t, []string{"t1"}, md.Get("x-tenant-id"))
}

func TestWithAuth_BearerOverrides(t *testing.T) {
	c := New(nil, nil, nil, WithSubject("alice"), WithBearerToken("jwt"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"Bearer jwt"}, md.Get("authorization"))
	assert.Empty(t, md.Get("x-subject"))
}

func TestWithAuth_Empty(t *testing.T) {
	c := New(nil, nil, nil, WithRole(""))
	ctx := c.withAuth(context.Background())
	_, ok := metadata.FromOutgoingContext(ctx)
	assert.False(t, ok)
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

	assert.Equal(t, "s1", s.ID)
	assert.Equal(t, "payments", s.Name)
	assert.Equal(t, int32(2), s.Version)
	assert.True(t, s.Published)
	assert.Len(t, s.Fields, 2)
	assert.Equal(t, "FIELD_TYPE_INT", s.Fields[0].Type)
	assert.Equal(t, "field desc", s.Fields[0].Description)
	assert.True(t, s.Fields[1].Nullable)
}

func TestSchemaFromProto_Nil(t *testing.T) {
	assert.Nil(t, schemaFromProto(nil))
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
			assert.Equal(t, tt.expected, result[0].Type)
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
	require.NotNil(t, c)
	assert.Equal(t, &min, c.Min)
	assert.Equal(t, &max, c.Max)
	assert.Equal(t, &minLen, c.MinLength)
	assert.Equal(t, "^[a-z]+$", *c.Regex)
	assert.Equal(t, []string{"a", "b"}, c.EnumValues)
	assert.Equal(t, `{"type":"object"}`, *c.JsonSchema)
}

func TestFieldsToProto_OptionalFields(t *testing.T) {
	result := fieldsToProto([]Field{{
		Path: "x", Type: "STRING",
		RedirectTo:  "y",
		Default:     "hello",
		Description: "a field",
	}})

	assert.Equal(t, "y", *result[0].RedirectTo)
	assert.Equal(t, "hello", *result[0].DefaultValue)
	assert.Equal(t, "a field", *result[0].Description)
}

func TestTenantFromProto(t *testing.T) {
	now := timestamppb.Now()
	tenant := tenantFromProto(&pb.Tenant{
		Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 2,
		CreatedAt: now, UpdatedAt: now,
	})
	assert.Equal(t, "acme", tenant.Name)
	assert.Equal(t, int32(2), tenant.SchemaVersion)
}

func TestTenantFromProto_Nil(t *testing.T) {
	assert.Nil(t, tenantFromProto(nil))
}

func TestVersionFromProto(t *testing.T) {
	v := versionFromProto(&pb.ConfigVersion{
		Id: "v1", TenantId: "t1", Version: 3, Description: "test",
		CreatedBy: "admin", CreatedAt: timestamppb.Now(),
	})
	assert.Equal(t, int32(3), v.Version)
	assert.Equal(t, "admin", v.CreatedBy)
}

func TestVersionFromProto_Nil(t *testing.T) {
	assert.Nil(t, versionFromProto(nil))
}

func TestAuditEntryFromProto(t *testing.T) {
	fp := "app.fee"
	old := "0.01"
	new := "0.02"
	ver := int32(3)
	e := auditEntryFromProto(&pb.AuditEntry{
		Id: "e1", TenantId: "t1", Actor: "admin", Action: "set_field",
		FieldPath: &fp, OldValue: &old, NewValue: &new,
		ConfigVersion: &ver, CreatedAt: timestamppb.Now(),
	})
	assert.Equal(t, "app.fee", e.FieldPath)
	assert.Equal(t, "0.01", e.OldValue)
	assert.Equal(t, int32(3), *e.ConfigVersion)
}

func TestAuditEntryFromProto_Nil(t *testing.T) {
	assert.Nil(t, auditEntryFromProto(nil))
}

func TestUsageStatsFromProto(t *testing.T) {
	lastBy := "reader"
	s := usageStatsFromProto(&pb.UsageStats{
		TenantId: "t1", FieldPath: "app.fee", ReadCount: 42,
		LastReadBy: &lastBy, LastReadAt: timestamppb.Now(),
	})
	assert.Equal(t, int64(42), s.ReadCount)
	assert.Equal(t, "reader", s.LastReadBy)
	assert.NotNil(t, s.LastReadAt)
}

func TestUsageStatsFromProto_Nil(t *testing.T) {
	assert.Nil(t, usageStatsFromProto(nil))
}

func TestPtrString(t *testing.T) {
	assert.Nil(t, ptrString(""))
	assert.Equal(t, "hello", *ptrString("hello"))
}

func TestTimeToProto(t *testing.T) {
	assert.Nil(t, timeToProto(nil))
	now := time.Now()
	assert.NotNil(t, timeToProto(&now))
}
