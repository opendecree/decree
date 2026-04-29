package schema

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

const (
	testSchemaID  = "00000000-0000-0000-0000-000000000001"
	testVersionID = "00000000-0000-0000-0000-000000000002"
	testTenantID  = "00000000-0000-0000-0000-000000000003"
)

// --- CreateSchema ---

func TestCreateSchema_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("CreateSchema", ctx, mock.AnythingOfType("schema.CreateSchemaParams")).
		Return(domain.Schema{ID: testSchemaID, Name: "test-schema"}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: testVersionID, SchemaID: testSchemaID, Version: 1, Checksum: "abc"}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("schema.CreateSchemaFieldParams")).
		Return(domain.SchemaField{Path: "test.field", FieldType: "string"}, nil)

	resp, err := svc.CreateSchema(ctx, &pb.CreateSchemaRequest{
		Name: "test-schema",
		Fields: []*pb.SchemaField{
			{Path: "test.field", Type: pb.FieldType_FIELD_TYPE_STRING},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "test-schema", resp.Schema.Name)
	assert.Equal(t, int32(1), resp.Schema.Version)
	assert.Len(t, resp.Schema.Fields, 1)
	store.AssertExpectations(t)
}

func TestCreateSchema_EmptyName(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	_, err := svc.CreateSchema(context.Background(), &pb.CreateSchemaRequest{Name: ""})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetSchema ---

func TestGetSchema_LatestVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "test"}, nil)
	store.On("GetLatestSchemaVersion", ctx, testSchemaID).
		Return(domain.SchemaVersion{ID: testVersionID, Version: 3, Published: true}, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{}, nil)

	resp, err := svc.GetSchema(ctx, &pb.GetSchemaRequest{Id: testSchemaID})

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestGetSchema_SpecificVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	v := int32(2)

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "test"}, nil)
	store.On("GetSchemaVersion", ctx, GetSchemaVersionParams{SchemaID: testSchemaID, Version: 2}).
		Return(domain.SchemaVersion{ID: testVersionID, Version: 2}, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{}, nil)

	resp, err := svc.GetSchema(ctx, &pb.GetSchemaRequest{Id: testSchemaID, Version: &v})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestGetSchema_NotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{}, domain.ErrNotFound)

	_, err := svc.GetSchema(ctx, &pb.GetSchemaRequest{Id: testSchemaID})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- UpdateSchema ---

func TestUpdateSchema_CreatesNewVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	oldVersionID := "00000000-0000-0000-0000-000000000010"
	newVersionID := "00000000-0000-0000-0000-000000000011"

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "test"}, nil)
	store.On("GetLatestSchemaVersion", ctx, testSchemaID).
		Return(domain.SchemaVersion{ID: oldVersionID, Version: 1, Published: true}, nil)
	store.On("GetSchemaFields", ctx, oldVersionID).
		Return([]domain.SchemaField{
			{Path: "existing.field", FieldType: "integer"},
		}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: newVersionID, Version: 2, ParentVersion: ptrInt32(1)}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("schema.CreateSchemaFieldParams")).
		Return(domain.SchemaField{}, nil)

	resp, err := svc.UpdateSchema(ctx, &pb.UpdateSchemaRequest{
		Id: testSchemaID,
		Fields: []*pb.SchemaField{
			{Path: "new.field", Type: pb.FieldType_FIELD_TYPE_STRING},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Schema.Version)
	// Should have 2 fields: existing + new
	store.AssertNumberOfCalls(t, "CreateSchemaField", 2)
	store.AssertExpectations(t)
}

// --- PublishSchema ---

func TestPublishSchema_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "test"}, nil)
	store.On("PublishSchemaVersion", ctx, PublishSchemaVersionParams{SchemaID: testSchemaID, Version: 1}).
		Return(domain.SchemaVersion{ID: testVersionID, Version: 1, Published: true}, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{}, nil)

	resp, err := svc.PublishSchema(ctx, &pb.PublishSchemaRequest{Id: testSchemaID, Version: 1})

	require.NoError(t, err)
	assert.True(t, resp.Schema.Published)
	store.AssertExpectations(t)
}

// --- CreateTenant ---

func TestCreateTenant_RequiresPublishedSchema(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaVersion", ctx, GetSchemaVersionParams{SchemaID: testSchemaID, Version: 1}).
		Return(domain.SchemaVersion{Published: false}, nil)

	_, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "test-tenant",
		SchemaId:      testSchemaID,
		SchemaVersion: 1,
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCreateTenant_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaVersion", ctx, GetSchemaVersionParams{SchemaID: testSchemaID, Version: 1}).
		Return(domain.SchemaVersion{Published: true}, nil)
	store.On("CreateTenant", ctx, mock.AnythingOfType("schema.CreateTenantParams")).
		Return(domain.Tenant{ID: testTenantID, Name: "test-tenant", SchemaID: testSchemaID, SchemaVersion: 1}, nil)

	resp, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "test-tenant",
		SchemaId:      testSchemaID,
		SchemaVersion: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, "test-tenant", resp.Tenant.Name)
	store.AssertExpectations(t)
}

// --- Field enrichment round-trip ---

func TestCreateSchema_FieldTagsNotPersisted(t *testing.T) {
	// BUG: tags and other enrichment attributes (title, format, example,
	// read_only, write_once, sensitive) are accepted by CreateSchema but
	// silently dropped because CreateSchemaFieldParams and domain.SchemaField
	// have no fields for them. After a round-trip (Create → Get), they're gone.
	//
	// This test will pass once the full chain is fixed:
	// CreateSchemaFieldParams → domain.SchemaField → DB → fieldToProto.

	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	title := "Fee Rate"
	format := "percentage"
	desc := "Fee percentage"

	inputField := &pb.SchemaField{
		Path:        "payments.fee_rate",
		Type:        pb.FieldType_FIELD_TYPE_NUMBER,
		Description: &desc,
		Title:       &title,
		Format:      &format,
		Tags:        []string{"billing", "critical"},
		ReadOnly:    true,
		Sensitive:   true,
	}

	// --- Create: verify the store receives enrichment attributes ---
	store.On("CreateSchema", ctx, mock.AnythingOfType("schema.CreateSchemaParams")).
		Return(domain.Schema{ID: testSchemaID, Name: "test-schema"}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: testVersionID, SchemaID: testSchemaID, Version: 1, Checksum: "abc"}, nil)
	store.On("CreateSchemaField", ctx, mock.MatchedBy(func(p CreateSchemaFieldParams) bool {
		return p.Path == "payments.fee_rate"
	})).Return(domain.SchemaField{
		Path:        "payments.fee_rate",
		FieldType:   "number",
		Description: &desc,
		Title:       &title,
		Format:      &format,
		Tags:        []string{"billing", "critical"},
		ReadOnly:    true,
		Sensitive:   true,
	}, nil)

	resp, err := svc.CreateSchema(ctx, &pb.CreateSchemaRequest{
		Name:   "test-schema",
		Fields: []*pb.SchemaField{inputField},
	})
	require.NoError(t, err)
	require.Len(t, resp.Schema.Fields, 1)

	got := resp.Schema.Fields[0]

	// These pass today.
	assert.Equal(t, "payments.fee_rate", got.Path)
	assert.Equal(t, &desc, got.Description)

	// These fail today — enrichment attributes lost in round-trip.
	assert.Equal(t, &title, got.Title, "title lost in round-trip")
	assert.Equal(t, &format, got.Format, "format lost in round-trip")
	assert.Equal(t, []string{"billing", "critical"}, got.Tags, "tags lost in round-trip")
	assert.True(t, got.ReadOnly, "read_only lost in round-trip")
	assert.True(t, got.Sensitive, "sensitive lost in round-trip")
}

// --- helpers ---

func ptrInt32(v int32) *int32 {
	return &v
}
