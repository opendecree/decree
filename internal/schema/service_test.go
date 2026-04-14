package schema

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func testUUID(b byte) pgtype.UUID {
	var id pgtype.UUID
	id.Bytes[0] = b
	id.Valid = true
	return id
}

// --- CreateSchema ---

func TestCreateSchema_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	versionID := testUUID(2)

	store.On("CreateSchema", ctx, mock.AnythingOfType("dbstore.CreateSchemaParams")).
		Return(dbstore.Schema{ID: schemaID, Name: "test-schema"}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("dbstore.CreateSchemaVersionParams")).
		Return(dbstore.SchemaVersion{ID: versionID, SchemaID: schemaID, Version: 1, Checksum: "abc"}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("dbstore.CreateSchemaFieldParams")).
		Return(dbstore.SchemaField{Path: "test.field", FieldType: "string"}, nil)

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
	svc := NewService(store, testLogger, nil, nil)

	_, err := svc.CreateSchema(context.Background(), &pb.CreateSchemaRequest{Name: ""})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetSchema ---

func TestGetSchema_LatestVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	versionID := testUUID(2)

	store.On("GetSchemaByID", ctx, schemaID).
		Return(dbstore.Schema{ID: schemaID, Name: "test"}, nil)
	store.On("GetLatestSchemaVersion", ctx, schemaID).
		Return(dbstore.SchemaVersion{ID: versionID, Version: 3, Published: true}, nil)
	store.On("GetSchemaFields", ctx, versionID).
		Return([]dbstore.SchemaField{}, nil)

	resp, err := svc.GetSchema(ctx, &pb.GetSchemaRequest{Id: uuidToString(schemaID)})

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestGetSchema_SpecificVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	versionID := testUUID(2)
	v := int32(2)

	store.On("GetSchemaByID", ctx, schemaID).
		Return(dbstore.Schema{ID: schemaID, Name: "test"}, nil)
	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 2}).
		Return(dbstore.SchemaVersion{ID: versionID, Version: 2}, nil)
	store.On("GetSchemaFields", ctx, versionID).
		Return([]dbstore.SchemaField{}, nil)

	resp, err := svc.GetSchema(ctx, &pb.GetSchemaRequest{Id: uuidToString(schemaID), Version: &v})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestGetSchema_NotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	store.On("GetSchemaByID", ctx, schemaID).
		Return(dbstore.Schema{}, pgx.ErrNoRows)

	_, err := svc.GetSchema(ctx, &pb.GetSchemaRequest{Id: uuidToString(schemaID)})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- UpdateSchema ---

func TestUpdateSchema_CreatesNewVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	oldVersionID := testUUID(2)
	newVersionID := testUUID(3)

	store.On("GetSchemaByID", ctx, schemaID).
		Return(dbstore.Schema{ID: schemaID, Name: "test"}, nil)
	store.On("GetLatestSchemaVersion", ctx, schemaID).
		Return(dbstore.SchemaVersion{ID: oldVersionID, Version: 1, Published: true}, nil)
	store.On("GetSchemaFields", ctx, oldVersionID).
		Return([]dbstore.SchemaField{
			{Path: "existing.field", FieldType: "int"},
		}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("dbstore.CreateSchemaVersionParams")).
		Return(dbstore.SchemaVersion{ID: newVersionID, Version: 2, ParentVersion: ptrInt32(1)}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("dbstore.CreateSchemaFieldParams")).
		Return(dbstore.SchemaField{}, nil)

	resp, err := svc.UpdateSchema(ctx, &pb.UpdateSchemaRequest{
		Id: uuidToString(schemaID),
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
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	versionID := testUUID(2)

	store.On("GetSchemaByID", ctx, schemaID).
		Return(dbstore.Schema{ID: schemaID, Name: "test"}, nil)
	store.On("PublishSchemaVersion", ctx, dbstore.PublishSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{ID: versionID, Version: 1, Published: true}, nil)
	store.On("GetSchemaFields", ctx, versionID).
		Return([]dbstore.SchemaField{}, nil)

	resp, err := svc.PublishSchema(ctx, &pb.PublishSchemaRequest{Id: uuidToString(schemaID), Version: 1})

	require.NoError(t, err)
	assert.True(t, resp.Schema.Published)
	store.AssertExpectations(t)
}

// --- CreateTenant ---

func TestCreateTenant_RequiresPublishedSchema(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)

	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{Published: false}, nil)

	_, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "test-tenant",
		SchemaId:      uuidToString(schemaID),
		SchemaVersion: 1,
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCreateTenant_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)
	ctx := context.Background()

	schemaID := testUUID(1)
	tenantID := testUUID(2)

	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{Published: true}, nil)
	store.On("CreateTenant", ctx, mock.AnythingOfType("dbstore.CreateTenantParams")).
		Return(dbstore.Tenant{ID: tenantID, Name: "test-tenant", SchemaID: schemaID, SchemaVersion: 1}, nil)

	resp, err := svc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          "test-tenant",
		SchemaId:      uuidToString(schemaID),
		SchemaVersion: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, "test-tenant", resp.Tenant.Name)
	store.AssertExpectations(t)
}

// --- helpers ---

func ptrInt32(v int32) *int32 {
	return &v
}
