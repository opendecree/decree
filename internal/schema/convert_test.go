package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

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

func TestDomainFieldTypeToProto_RoundTrip(t *testing.T) {
	types := map[domain.FieldType]pb.FieldType{
		domain.FieldTypeInteger:  pb.FieldType_FIELD_TYPE_INT,
		domain.FieldTypeNumber:   pb.FieldType_FIELD_TYPE_NUMBER,
		domain.FieldTypeString:   pb.FieldType_FIELD_TYPE_STRING,
		domain.FieldTypeBool:     pb.FieldType_FIELD_TYPE_BOOL,
		domain.FieldTypeTime:     pb.FieldType_FIELD_TYPE_TIME,
		domain.FieldTypeDuration: pb.FieldType_FIELD_TYPE_DURATION,
		domain.FieldTypeURL:      pb.FieldType_FIELD_TYPE_URL,
		domain.FieldTypeJSON:     pb.FieldType_FIELD_TYPE_JSON,
	}
	for domainType, protoType := range types {
		assert.Equal(t, protoType, domainType.ToProto(), "domainType: %s", domainType)
		assert.Equal(t, domainType, domain.FieldTypeFromProto(protoType), "protoType: %s", protoType)
	}
}

func TestPtrString(t *testing.T) {
	assert.Nil(t, ptrString(""))
	s := ptrString("hello")
	assert.NotNil(t, s)
	assert.Equal(t, "hello", *s)
}
