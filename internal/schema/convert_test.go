package schema

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

func TestParseUUID_Valid(t *testing.T) {
	id, err := parseUUID("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.True(t, id.Valid)
}

func TestParseUUID_Invalid(t *testing.T) {
	_, err := parseUUID("not-a-uuid")
	require.Error(t, err)
}

func TestUUIDToString_Roundtrip(t *testing.T) {
	original := testUUID(42)
	str := uuidToString(original)
	parsed, err := parseUUID(str)
	require.NoError(t, err)
	assert.Equal(t, original.Bytes, parsed.Bytes)
}

func TestUUIDToString_Invalid(t *testing.T) {
	var id pgtype.UUID
	assert.Equal(t, "", uuidToString(id))
}

func TestComputeChecksum_Deterministic(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "b.field", Type: pb.FieldType_FIELD_TYPE_INT},
		{Path: "a.field", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	c1 := computeChecksum(fields)
	c2 := computeChecksum(fields)
	assert.Equal(t, c1, c2)
}

func TestComputeChecksum_OrderIndependent(t *testing.T) {
	fields1 := []*pb.SchemaField{
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_INT},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	fields2 := []*pb.SchemaField{
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_INT},
	}
	assert.Equal(t, computeChecksum(fields1), computeChecksum(fields2))
}

func TestFieldTypeToProto_RoundTrip(t *testing.T) {
	types := map[dbstore.FieldType]pb.FieldType{
		dbstore.FieldTypeInteger:  pb.FieldType_FIELD_TYPE_INT,
		dbstore.FieldTypeNumber:   pb.FieldType_FIELD_TYPE_NUMBER,
		dbstore.FieldTypeString:   pb.FieldType_FIELD_TYPE_STRING,
		dbstore.FieldTypeBool:     pb.FieldType_FIELD_TYPE_BOOL,
		dbstore.FieldTypeTime:     pb.FieldType_FIELD_TYPE_TIME,
		dbstore.FieldTypeDuration: pb.FieldType_FIELD_TYPE_DURATION,
		dbstore.FieldTypeUrl:      pb.FieldType_FIELD_TYPE_URL,
		dbstore.FieldTypeJson:     pb.FieldType_FIELD_TYPE_JSON,
	}
	for dbType, protoType := range types {
		assert.Equal(t, protoType, fieldTypeToProto(dbType), "dbType: %s", dbType)
		assert.Equal(t, dbType, protoFieldType(protoType), "protoType: %s", protoType)
	}
}

func TestPtrString(t *testing.T) {
	assert.Nil(t, ptrString(""))
	s := ptrString("hello")
	require.NotNil(t, s)
	assert.Equal(t, "hello", *s)
}
