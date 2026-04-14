package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

// parseUUID parses a string UUID into pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return id, nil
}

// uuidToString converts pgtype.UUID to string.
func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

// computeChecksum computes a checksum for a config value using xxHash.
func computeChecksum(value string) string {
	h := xxhash.Sum64String(value)
	return strconv.FormatUint(h, 16)
}

// checksumPtr computes a checksum for a *string value. Returns nil for nil input.
func checksumPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cs := computeChecksum(*value)
	return &cs
}

// configVersionToProto converts a DB config version to proto.
func configVersionToProto(v dbstore.ConfigVersion) *pb.ConfigVersion {
	result := &pb.ConfigVersion{
		Id:        uuidToString(v.ID),
		TenantId:  uuidToString(v.TenantID),
		Version:   v.Version,
		CreatedBy: v.CreatedBy,
		CreatedAt: timestamppb.New(v.CreatedAt.Time),
	}
	if v.Description != nil {
		result.Description = *v.Description
	}
	return result
}

// ptrString returns a pointer to s, or nil if empty.
func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// strPtr returns a pointer to s (always non-nil, even for empty string).
func strPtr(s string) *string {
	return &s
}

// derefString safely dereferences a *string, returning "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// --- TypedValue ↔ string conversion ---

// typedValueToString serializes a TypedValue to its string representation for DB storage.
// Returns nil for a nil (null) TypedValue.
func typedValueToString(tv *pb.TypedValue) *string {
	if tv == nil {
		return nil
	}
	var s string
	switch v := tv.Kind.(type) {
	case *pb.TypedValue_IntegerValue:
		s = strconv.FormatInt(v.IntegerValue, 10)
	case *pb.TypedValue_NumberValue:
		s = strconv.FormatFloat(v.NumberValue, 'f', -1, 64)
	case *pb.TypedValue_StringValue:
		s = v.StringValue
	case *pb.TypedValue_BoolValue:
		s = strconv.FormatBool(v.BoolValue)
	case *pb.TypedValue_TimeValue:
		if v.TimeValue != nil {
			s = v.TimeValue.AsTime().Format(time.RFC3339Nano)
		}
	case *pb.TypedValue_DurationValue:
		if v.DurationValue != nil {
			s = v.DurationValue.AsDuration().String()
		}
	case *pb.TypedValue_UrlValue:
		s = v.UrlValue
	case *pb.TypedValue_JsonValue:
		s = v.JsonValue
	default:
		return nil
	}
	return &s
}

// stringToTypedValue converts a DB string value to a TypedValue using the field type.
// Returns nil for a nil (null) string.
func stringToTypedValue(s *string, ft pb.FieldType) *pb.TypedValue { //nolint:unparam // ft will vary once field-type-aware reads are added
	if s == nil {
		return nil
	}
	switch ft {
	case pb.FieldType_FIELD_TYPE_INT:
		v, _ := strconv.ParseInt(*s, 10, 64)
		return &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: v}}
	case pb.FieldType_FIELD_TYPE_NUMBER:
		v, _ := strconv.ParseFloat(*s, 64)
		return &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: v}}
	case pb.FieldType_FIELD_TYPE_STRING:
		return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: *s}}
	case pb.FieldType_FIELD_TYPE_BOOL:
		v, _ := strconv.ParseBool(*s)
		return &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: v}}
	case pb.FieldType_FIELD_TYPE_TIME:
		t, _ := time.Parse(time.RFC3339Nano, *s)
		return &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(t)}}
	case pb.FieldType_FIELD_TYPE_DURATION:
		d, _ := time.ParseDuration(*s)
		return &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(d)}}
	case pb.FieldType_FIELD_TYPE_URL:
		return &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: *s}}
	case pb.FieldType_FIELD_TYPE_JSON:
		return &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: *s}}
	default:
		return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: *s}}
	}
}

// typedValueChecksum computes a checksum for a TypedValue by serializing to string first.
func typedValueChecksum(tv *pb.TypedValue) string { //nolint:unused // used in Phase 2 validation
	s := typedValueToString(tv)
	if s == nil {
		return computeChecksum("")
	}
	return computeChecksum(*s)
}

// typedValueToDisplayString returns a human-readable string for a TypedValue (for audit/events).
func typedValueToDisplayString(tv *pb.TypedValue) string {
	s := typedValueToString(tv)
	if s == nil {
		return ""
	}
	return *s
}

// validateTypedValueType checks that a TypedValue matches the expected field type.
func validateTypedValueType(tv *pb.TypedValue, expected pb.FieldType) error { //nolint:unused // used in Phase 2 validation
	if tv == nil {
		return nil // null is valid for any type (nullable check is separate)
	}
	switch expected {
	case pb.FieldType_FIELD_TYPE_INT:
		if _, ok := tv.Kind.(*pb.TypedValue_IntegerValue); !ok {
			return fmt.Errorf("expected integer value")
		}
	case pb.FieldType_FIELD_TYPE_NUMBER:
		if _, ok := tv.Kind.(*pb.TypedValue_NumberValue); !ok {
			return fmt.Errorf("expected number value")
		}
	case pb.FieldType_FIELD_TYPE_STRING:
		if _, ok := tv.Kind.(*pb.TypedValue_StringValue); !ok {
			return fmt.Errorf("expected string value")
		}
	case pb.FieldType_FIELD_TYPE_BOOL:
		if _, ok := tv.Kind.(*pb.TypedValue_BoolValue); !ok {
			return fmt.Errorf("expected bool value")
		}
	case pb.FieldType_FIELD_TYPE_TIME:
		if _, ok := tv.Kind.(*pb.TypedValue_TimeValue); !ok {
			return fmt.Errorf("expected time value")
		}
	case pb.FieldType_FIELD_TYPE_DURATION:
		if _, ok := tv.Kind.(*pb.TypedValue_DurationValue); !ok {
			return fmt.Errorf("expected duration value")
		}
	case pb.FieldType_FIELD_TYPE_URL:
		if v, ok := tv.Kind.(*pb.TypedValue_UrlValue); ok {
			u, err := url.Parse(v.UrlValue)
			if err != nil || !u.IsAbs() {
				return fmt.Errorf("invalid absolute URL")
			}
		} else {
			return fmt.Errorf("expected url value")
		}
	case pb.FieldType_FIELD_TYPE_JSON:
		if v, ok := tv.Kind.(*pb.TypedValue_JsonValue); ok {
			if !json.Valid([]byte(v.JsonValue)) {
				return fmt.Errorf("invalid JSON")
			}
		} else {
			return fmt.Errorf("expected json value")
		}
	}
	return nil
}

// dbFieldTypeToProto converts a DB field type to proto enum.
func dbFieldTypeToProto(t dbstore.FieldType) pb.FieldType {
	switch t {
	case dbstore.FieldTypeInteger:
		return pb.FieldType_FIELD_TYPE_INT
	case dbstore.FieldTypeNumber:
		return pb.FieldType_FIELD_TYPE_NUMBER
	case dbstore.FieldTypeString:
		return pb.FieldType_FIELD_TYPE_STRING
	case dbstore.FieldTypeBool:
		return pb.FieldType_FIELD_TYPE_BOOL
	case dbstore.FieldTypeTime:
		return pb.FieldType_FIELD_TYPE_TIME
	case dbstore.FieldTypeDuration:
		return pb.FieldType_FIELD_TYPE_DURATION
	case dbstore.FieldTypeUrl:
		return pb.FieldType_FIELD_TYPE_URL
	case dbstore.FieldTypeJson:
		return pb.FieldType_FIELD_TYPE_JSON
	default:
		return pb.FieldType_FIELD_TYPE_UNSPECIFIED
	}
}
