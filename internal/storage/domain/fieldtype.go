package domain

import (
	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// ToProto converts a domain FieldType to the corresponding proto enum value.
func (ft FieldType) ToProto() pb.FieldType {
	switch ft {
	case FieldTypeInteger:
		return pb.FieldType_FIELD_TYPE_INT
	case FieldTypeNumber:
		return pb.FieldType_FIELD_TYPE_NUMBER
	case FieldTypeString:
		return pb.FieldType_FIELD_TYPE_STRING
	case FieldTypeBool:
		return pb.FieldType_FIELD_TYPE_BOOL
	case FieldTypeTime:
		return pb.FieldType_FIELD_TYPE_TIME
	case FieldTypeDuration:
		return pb.FieldType_FIELD_TYPE_DURATION
	case FieldTypeURL:
		return pb.FieldType_FIELD_TYPE_URL
	case FieldTypeJSON:
		return pb.FieldType_FIELD_TYPE_JSON
	default:
		return pb.FieldType_FIELD_TYPE_UNSPECIFIED
	}
}

// FieldTypeFromProto converts a proto FieldType enum to the domain FieldType.
func FieldTypeFromProto(t pb.FieldType) FieldType {
	switch t {
	case pb.FieldType_FIELD_TYPE_INT:
		return FieldTypeInteger
	case pb.FieldType_FIELD_TYPE_NUMBER:
		return FieldTypeNumber
	case pb.FieldType_FIELD_TYPE_STRING:
		return FieldTypeString
	case pb.FieldType_FIELD_TYPE_BOOL:
		return FieldTypeBool
	case pb.FieldType_FIELD_TYPE_TIME:
		return FieldTypeTime
	case pb.FieldType_FIELD_TYPE_DURATION:
		return FieldTypeDuration
	case pb.FieldType_FIELD_TYPE_URL:
		return FieldTypeURL
	case pb.FieldType_FIELD_TYPE_JSON:
		return FieldTypeJSON
	default:
		return FieldTypeString
	}
}
