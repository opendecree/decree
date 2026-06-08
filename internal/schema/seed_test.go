package schema

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// publishedSchemaWithFields seeds the memory store with a schema, a published
// version 1, and the given fields, returning the schema ID. It drives the same
// service entry points a real caller would, so the resulting version carries a
// proper ID and the fields are stored under it.
func publishedSchemaWithFields(t *testing.T, svc *Service, name string, fields []*pb.SchemaField) string {
	t.Helper()
	ctx := superadminCtx()

	resp, err := svc.CreateSchema(ctx, &pb.CreateSchemaRequest{Name: name, Fields: fields})
	require.NoError(t, err)
	_, err = svc.PublishSchema(ctx, &pb.PublishSchemaRequest{Id: resp.Schema.Id, Version: 1})
	require.NoError(t, err)
	return resp.Schema.Id
}

func strptr(s string) *string { return &s }

func TestCreateTenant_SeedsSchemaDefaults(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	// Two fields carry defaults; one does not.
	schemaID := publishedSchemaWithFields(t, svc, "with-defaults", []*pb.SchemaField{
		{Path: "a.retries", Type: pb.FieldType_FIELD_TYPE_INT, DefaultValue: strptr("3")},
		{Path: "a.enabled", Type: pb.FieldType_FIELD_TYPE_BOOL, DefaultValue: strptr("true")},
		{Path: "a.label", Type: pb.FieldType_FIELD_TYPE_STRING}, // no default
	})

	tenantResp, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "acme",
		SchemaId:      schemaID,
		SchemaVersion: 1,
	})
	require.NoError(t, err)
	tenantID := tenantResp.Tenant.Id

	// Exactly one config version (version 1) seeded for this tenant.
	versions := store.ConfigVersionsForTenant(tenantID)
	require.Len(t, versions, 1)
	assert.Equal(t, int32(1), versions[0].Version)

	// Values contain exactly the two defaults; the field without a default is unset.
	values := store.ConfigValuesForTenant(tenantID)
	assert.Equal(t, map[string]string{
		"a.retries": "3",
		"a.enabled": "true",
	}, values)
	_, hasLabel := values["a.label"]
	assert.False(t, hasLabel, "field without a default must remain unset")
}

func TestCreateTenant_NoDefaults_NoSeededVersion(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	schemaID := publishedSchemaWithFields(t, svc, "no-defaults", []*pb.SchemaField{
		{Path: "x.host", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "x.port", Type: pb.FieldType_FIELD_TYPE_INT},
	})

	tenantResp, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "globex",
		SchemaId:      schemaID,
		SchemaVersion: 1,
	})
	require.NoError(t, err)

	// No defaults → no version 1 is created (unchanged pre-defaults behavior).
	versions := store.ConfigVersionsForTenant(tenantResp.Tenant.Id)
	assert.Empty(t, versions)
	assert.Empty(t, store.ConfigValuesForTenant(tenantResp.Tenant.Id))
}

func TestCreateTenant_InvalidDefault_Rejected(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	// Default "abc" is not a valid integer → tenant creation must fail and seed nothing.
	schemaID := publishedSchemaWithFields(t, svc, "bad-default", []*pb.SchemaField{
		{Path: "n.count", Type: pb.FieldType_FIELD_TYPE_INT, DefaultValue: strptr("abc")},
	})

	_, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "initech",
		SchemaId:      schemaID,
		SchemaVersion: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	// No tenant row and no config should have been committed.
	_, getErr := store.GetTenantByName(ctx, "initech")
	assert.ErrorIs(t, getErr, domain.ErrNotFound)
}

func TestCreateTenant_DefaultViolatingConstraint_Rejected(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	// minimum=1, default=0 → constraint violation rejected at tenant creation.
	min := float64(1)
	schemaID := publishedSchemaWithFields(t, svc, "constrained-default", []*pb.SchemaField{
		{
			Path:         "n.count",
			Type:         pb.FieldType_FIELD_TYPE_INT,
			Constraints:  &pb.FieldConstraints{Min: &min},
			DefaultValue: strptr("0"),
		},
	})

	_, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "umbrella",
		SchemaId:      schemaID,
		SchemaVersion: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// --- focused unit coverage for the default-collection helper ---

func TestCollectDefaultValues(t *testing.T) {
	min := float64(1)
	cJSON, err := json.Marshal(&pb.FieldConstraints{Min: &min})
	require.NoError(t, err)

	t.Run("collects only fields with defaults and checksums them", func(t *testing.T) {
		out, err := collectDefaultValues([]domain.SchemaField{
			{Path: "a", FieldType: domain.FieldTypeInteger, DefaultValue: strptr("5")},
			{Path: "b", FieldType: domain.FieldTypeString}, // no default
		})
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "5", out["a"].Value)
		assert.Equal(t, configValueChecksum("5"), out["a"].Checksum)
	})

	t.Run("nil when no field has a default", func(t *testing.T) {
		out, err := collectDefaultValues([]domain.SchemaField{
			{Path: "a", FieldType: domain.FieldTypeInteger},
		})
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("rejects unparseable default", func(t *testing.T) {
		_, err := collectDefaultValues([]domain.SchemaField{
			{Path: "a", FieldType: domain.FieldTypeInteger, DefaultValue: strptr("notnum")},
		})
		require.Error(t, err)
	})

	t.Run("rejects constraint-violating default", func(t *testing.T) {
		_, err := collectDefaultValues([]domain.SchemaField{
			{Path: "a", FieldType: domain.FieldTypeInteger, Constraints: cJSON, DefaultValue: strptr("0")},
		})
		require.Error(t, err)
	})

	t.Run("nullable string default is valid", func(t *testing.T) {
		out, err := collectDefaultValues([]domain.SchemaField{
			{Path: "a", FieldType: domain.FieldTypeString, Nullable: true, DefaultValue: strptr("hi")},
		})
		require.NoError(t, err)
		assert.Equal(t, "hi", out["a"].Value)
	})
}

func TestDefaultToTypedValue(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		ft      domain.FieldType
		wantErr bool
	}{
		{"int ok", "42", domain.FieldTypeInteger, false},
		{"int bad", "x", domain.FieldTypeInteger, true},
		{"number ok", "1.5", domain.FieldTypeNumber, false},
		{"number bad", "x", domain.FieldTypeNumber, true},
		{"bool ok", "true", domain.FieldTypeBool, false},
		{"bool bad", "yes", domain.FieldTypeBool, true},
		{"string ok", "hi", domain.FieldTypeString, false},
		{"time ok", "2024-01-02T03:04:05Z", domain.FieldTypeTime, false},
		{"time bad", "nope", domain.FieldTypeTime, true},
		{"duration ok", "5s", domain.FieldTypeDuration, false},
		{"duration bad", "5", domain.FieldTypeDuration, true},
		{"url passthrough", "https://x", domain.FieldTypeURL, false},
		{"json passthrough", "{}", domain.FieldTypeJSON, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := defaultToTypedValue(tc.in, tc.ft)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryStore_SeedTenantConfig_EmptyIsNoop(t *testing.T) {
	store := NewMemoryStore()
	err := store.SeedTenantConfig(context.Background(), SeedTenantConfigParams{
		TenantID: testTenantID,
		Actor:    "tester",
		Values:   nil,
	})
	require.NoError(t, err)
	assert.Empty(t, store.ConfigVersionsForTenant(testTenantID))
}
