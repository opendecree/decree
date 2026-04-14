package configclient

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// --- String getters (always work, convert any type to string) ---

// Get returns the current value of a single configuration field as a string.
// Any typed value is converted to its string representation.
// Returns [ErrNotFound] if the field has no value set.
func (c *Client) Get(ctx context.Context, tenantID, fieldPath string) (string, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return "", err
	}
	return typedValueToString(tv), nil
}

// GetAll returns all configuration values for a tenant as a map of field path to string value.
// Returns an empty map if no values are set.
func (c *Client) GetAll(ctx context.Context, tenantID string) (map[string]string, error) {
	resp, err := c.rpc.GetConfig(c.withAuth(ctx), &pb.GetConfigRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return configToMap(resp.Config), nil
}

// GetFields returns the string values for the specified field paths.
// Fields that have no value set are omitted from the result.
func (c *Client) GetFields(ctx context.Context, tenantID string, fieldPaths []string) (map[string]string, error) {
	resp, err := c.rpc.GetFields(c.withAuth(ctx), &pb.GetFieldsRequest{
		TenantId:   tenantID,
		FieldPaths: fieldPaths,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make(map[string]string, len(resp.Values))
	for _, v := range resp.Values {
		result[v.FieldPath] = typedValueToString(v.Value)
	}
	return result, nil
}

// --- Type-specific getters ---

// GetString returns the current value as a string.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not a string type.
func (c *Client) GetString(ctx context.Context, tenantID, fieldPath string) (string, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return "", err
	}
	if tv == nil {
		return "", nil
	}
	switch v := tv.Kind.(type) {
	case *pb.TypedValue_StringValue:
		return v.StringValue, nil
	case *pb.TypedValue_UrlValue:
		return v.UrlValue, nil
	case *pb.TypedValue_JsonValue:
		return v.JsonValue, nil
	default:
		return "", ErrTypeMismatch
	}
}

// GetInt returns the current value as an int64.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not an integer type.
func (c *Client) GetInt(ctx context.Context, tenantID, fieldPath string) (int64, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return 0, err
	}
	if tv == nil {
		return 0, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_IntegerValue); ok {
		return v.IntegerValue, nil
	}
	return 0, ErrTypeMismatch
}

// GetFloat returns the current value as a float64.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not a number type.
func (c *Client) GetFloat(ctx context.Context, tenantID, fieldPath string) (float64, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return 0, err
	}
	if tv == nil {
		return 0, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_NumberValue); ok {
		return v.NumberValue, nil
	}
	return 0, ErrTypeMismatch
}

// GetBool returns the current value as a bool.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not a bool type.
func (c *Client) GetBool(ctx context.Context, tenantID, fieldPath string) (bool, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return false, err
	}
	if tv == nil {
		return false, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_BoolValue); ok {
		return v.BoolValue, nil
	}
	return false, ErrTypeMismatch
}

// GetTime returns the current value as a time.Time.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not a time type.
func (c *Client) GetTime(ctx context.Context, tenantID, fieldPath string) (time.Time, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return time.Time{}, err
	}
	if tv == nil {
		return time.Time{}, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_TimeValue); ok && v.TimeValue != nil {
		return v.TimeValue.AsTime(), nil
	}
	return time.Time{}, ErrTypeMismatch
}

// GetDuration returns the current value as a time.Duration.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not a duration type.
func (c *Client) GetDuration(ctx context.Context, tenantID, fieldPath string) (time.Duration, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return 0, err
	}
	if tv == nil {
		return 0, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_DurationValue); ok && v.DurationValue != nil {
		return v.DurationValue.AsDuration(), nil
	}
	return 0, ErrTypeMismatch
}

// --- Nullable getters (nil = null) ---

// GetStringNullable returns the string value or nil if null.
// Returns [ErrNotFound] if the field has no value set.
func (c *Client) GetStringNullable(ctx context.Context, tenantID, fieldPath string) (*string, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	s := typedValueToString(tv)
	return &s, nil
}

// GetIntNullable returns the int64 value or nil if null.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not an integer type.
func (c *Client) GetIntNullable(ctx context.Context, tenantID, fieldPath string) (*int64, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_IntegerValue); ok {
		return &v.IntegerValue, nil
	}
	return nil, ErrTypeMismatch
}

// GetBoolNullable returns the bool value or nil if null.
// Returns [ErrNotFound] if the field has no value set.
// Returns [ErrTypeMismatch] if the field is not a bool type.
func (c *Client) GetBoolNullable(ctx context.Context, tenantID, fieldPath string) (*bool, error) {
	tv, err := c.getTypedValue(ctx, tenantID, fieldPath)
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	if v, ok := tv.Kind.(*pb.TypedValue_BoolValue); ok {
		return &v.BoolValue, nil
	}
	return nil, ErrTypeMismatch
}

// --- Type-specific setters ---

// SetInt writes an integer configuration value.
func (c *Client) SetInt(ctx context.Context, tenantID, fieldPath string, value int64) error {
	return c.setTyped(ctx, tenantID, fieldPath, &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: value}})
}

// SetFloat writes a floating-point configuration value.
func (c *Client) SetFloat(ctx context.Context, tenantID, fieldPath string, value float64) error {
	return c.setTyped(ctx, tenantID, fieldPath, &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: value}})
}

// SetBool writes a boolean configuration value.
func (c *Client) SetBool(ctx context.Context, tenantID, fieldPath string, value bool) error {
	return c.setTyped(ctx, tenantID, fieldPath, &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: value}})
}

// SetTime writes a timestamp configuration value.
func (c *Client) SetTime(ctx context.Context, tenantID, fieldPath string, value time.Time) error {
	return c.setTyped(ctx, tenantID, fieldPath, &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(value)}})
}

// SetDuration writes a duration configuration value.
func (c *Client) SetDuration(ctx context.Context, tenantID, fieldPath string, value time.Duration) error {
	return c.setTyped(ctx, tenantID, fieldPath, &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(value)}})
}

// --- Internal helpers ---

func (c *Client) getTypedValue(ctx context.Context, tenantID, fieldPath string) (*pb.TypedValue, error) {
	resp, err := c.rpc.GetField(c.withAuth(ctx), &pb.GetFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return resp.Value.Value, nil
}

func (c *Client) setTyped(ctx context.Context, tenantID, fieldPath string, value *pb.TypedValue) error {
	_, err := c.rpc.SetField(c.withAuth(ctx), &pb.SetFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
		Value:     value,
	})
	return mapError(err)
}

func configToMap(cfg *pb.Config) map[string]string {
	if cfg == nil {
		return nil
	}
	m := make(map[string]string, len(cfg.Values))
	for _, v := range cfg.Values {
		m[v.FieldPath] = typedValueToString(v.Value)
	}
	return m
}

// typedValueToString extracts a string representation from a TypedValue.
func typedValueToString(tv *pb.TypedValue) string {
	if tv == nil {
		return ""
	}
	switch v := tv.Kind.(type) {
	case *pb.TypedValue_StringValue:
		return v.StringValue
	case *pb.TypedValue_IntegerValue:
		return fmt.Sprintf("%d", v.IntegerValue)
	case *pb.TypedValue_NumberValue:
		return strconv.FormatFloat(v.NumberValue, 'f', -1, 64)
	case *pb.TypedValue_BoolValue:
		return strconv.FormatBool(v.BoolValue)
	case *pb.TypedValue_UrlValue:
		return v.UrlValue
	case *pb.TypedValue_JsonValue:
		return v.JsonValue
	case *pb.TypedValue_TimeValue:
		if v.TimeValue != nil {
			return v.TimeValue.AsTime().Format(time.RFC3339Nano)
		}
		return ""
	case *pb.TypedValue_DurationValue:
		if v.DurationValue != nil {
			return v.DurationValue.AsDuration().String()
		}
		return ""
	default:
		return ""
	}
}

// StringValue creates a TypedValue wrapping a string.
func StringValue(s string) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: s}}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
