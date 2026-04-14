package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestFieldType_ToProto_RoundTrip(t *testing.T) {
	types := map[FieldType]pb.FieldType{
		FieldTypeInteger:  pb.FieldType_FIELD_TYPE_INT,
		FieldTypeNumber:   pb.FieldType_FIELD_TYPE_NUMBER,
		FieldTypeString:   pb.FieldType_FIELD_TYPE_STRING,
		FieldTypeBool:     pb.FieldType_FIELD_TYPE_BOOL,
		FieldTypeTime:     pb.FieldType_FIELD_TYPE_TIME,
		FieldTypeDuration: pb.FieldType_FIELD_TYPE_DURATION,
		FieldTypeURL:      pb.FieldType_FIELD_TYPE_URL,
		FieldTypeJSON:     pb.FieldType_FIELD_TYPE_JSON,
	}

	for domainType, protoType := range types {
		t.Run(string(domainType), func(t *testing.T) {
			assert.Equal(t, protoType, domainType.ToProto())
			assert.Equal(t, domainType, FieldTypeFromProto(protoType))
		})
	}
}

func TestFieldType_ToProto_Unknown(t *testing.T) {
	assert.Equal(t, pb.FieldType_FIELD_TYPE_UNSPECIFIED, FieldType("unknown").ToProto())
}

func TestFieldTypeFromProto_Unknown(t *testing.T) {
	assert.Equal(t, FieldTypeString, FieldTypeFromProto(pb.FieldType_FIELD_TYPE_UNSPECIFIED))
}

func TestErrNotFound(t *testing.T) {
	assert.NotNil(t, ErrNotFound)
	assert.Equal(t, "not found", ErrNotFound.Error())
}
