package schema

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	assert.Equal(t, 10_000, l.MaxFields)
	assert.Equal(t, 5*1024*1024, l.MaxDocBytes)
}

func TestCreateSchema_ExceedsMaxFields(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store,
		WithLogger(testLogger),
		WithLimits(Limits{MaxFields: 2}),
	)

	fields := []*pb.SchemaField{
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "c", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	_, err := svc.CreateSchema(context.Background(), &pb.CreateSchemaRequest{
		Name:   "too-big",
		Fields: fields,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 2")
	store.AssertNotCalled(t, "CreateSchema")
}

func TestCreateSchema_AtLimitAllowed(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store,
		WithLogger(testLogger),
		WithLimits(Limits{MaxFields: 2}),
	)

	store.On("CreateSchema", mock.Anything, mock.Anything).
		Return(domain.Schema{ID: testSchemaID, Name: "ok"}, nil)
	store.On("CreateSchemaVersion", mock.Anything, mock.Anything).
		Return(domain.SchemaVersion{ID: testVersionID, SchemaID: testSchemaID, Version: 1}, nil)
	store.On("CreateSchemaField", mock.Anything, mock.Anything).
		Return(domain.SchemaField{Path: "a", FieldType: "string"}, nil)

	_, err := svc.CreateSchema(context.Background(), &pb.CreateSchemaRequest{
		Name: "ok",
		Fields: []*pb.SchemaField{
			{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING},
			{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING},
		},
	})
	require.NoError(t, err)
}

func TestImportSchema_ExceedsMaxDocBytes(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store,
		WithLogger(testLogger),
		WithLimits(Limits{MaxDocBytes: 100}),
	)

	yaml := []byte(strings.Repeat("x", 200))
	_, err := svc.ImportSchema(context.Background(), &pb.ImportSchemaRequest{
		YamlContent: yaml,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 100")
}

func TestImportSchema_ExceedsMaxFields(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store,
		WithLogger(testLogger),
		WithLimits(Limits{MaxFields: 1}),
	)

	yaml := []byte("spec_version: v1\nname: too-many\nfields:\n  a:\n    type: string\n  b:\n    type: string\n")
	_, err := svc.ImportSchema(context.Background(), &pb.ImportSchemaRequest{
		YamlContent: yaml,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 1")
}
