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

func TestFieldToProto_EnrichmentAttributes_RoundTrip(t *testing.T) {
	// BUG: domain.SchemaField has no fields for tags, title, format, example,
	// read_only, write_once, or sensitive. fieldToProto can never return them,
	// so any enrichment attributes set on the proto are silently dropped after
	// a round-trip through storage.
	//
	// This test documents the bug: it will pass once domain.SchemaField and
	// fieldToProto are updated to carry these attributes.

	desc := "Fee percentage"
	title := "Fee Rate"
	example := "0.15"
	format := "percentage"

	domainField := domain.SchemaField{
		ID:          "field-1",
		Path:        "payments.fee_rate",
		FieldType:   domain.FieldTypeNumber,
		Description: &desc,
		Title:       &title,
		Example:     &example,
		Format:      &format,
		Tags:        []string{"billing", "critical"},
		ReadOnly:    true,
		WriteOnce:   true,
		Sensitive:   true,
	}

	got := fieldToProto(domainField)

	// These pass today — original attributes survive.
	assert.Equal(t, "payments.fee_rate", got.Path)
	assert.Equal(t, pb.FieldType_FIELD_TYPE_NUMBER, got.Type)
	assert.Equal(t, &desc, got.Description)

	// These fail today — enrichment attributes are not carried by domain.SchemaField.
	assert.Equal(t, &title, got.Title, "title should survive round-trip")
	assert.Equal(t, &example, got.Example, "example should survive round-trip")
	assert.Equal(t, &format, got.Format, "format should survive round-trip")
	assert.Equal(t, []string{"billing", "critical"}, got.Tags, "tags should survive round-trip")
	assert.True(t, got.ReadOnly, "read_only should survive round-trip")
	assert.True(t, got.WriteOnce, "write_once should survive round-trip")
	assert.True(t, got.Sensitive, "sensitive should survive round-trip")
}

func TestPtrString(t *testing.T) {
	assert.Nil(t, ptrString(""))
	s := ptrString("hello")
	assert.NotNil(t, s)
	assert.Equal(t, "hello", *s)
}
