package configclient

import (
	"context"
	"time"
)

// --- String getters (always work, convert any type to string) ---

// Get returns the current value of a single configuration field as a string.
// Any typed value is converted to its string representation.
// Returns [ErrNotFound] if the field has no value set.
func (c *Client) Get(ctx context.Context, tenantID, fieldPath string) (string, error) {
	return retry(ctx, c, func(ctx context.Context) (string, error) {
		resp, err := c.transport.GetField(ctx, &GetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
		})
		if err != nil {
			return "", err
		}
		return resp.Value.String(), nil
	})
}

// GetAll returns all configuration values for a tenant as a map of field path to string value.
// Returns an empty map if no values are set.
func (c *Client) GetAll(ctx context.Context, tenantID string) (map[string]string, error) {
	return retry(ctx, c, func(ctx context.Context) (map[string]string, error) {
		resp, err := c.transport.GetConfig(ctx, &GetConfigRequest{
			TenantID: tenantID,
		})
		if err != nil {
			return nil, err
		}
		return configToMap(resp), nil
	})
}

// GetFields returns the string values for the specified field paths.
// Fields that have no value set are omitted from the result.
func (c *Client) GetFields(ctx context.Context, tenantID string, fieldPaths []string) (map[string]string, error) {
	return retry(ctx, c, func(ctx context.Context) (map[string]string, error) {
		resp, err := c.transport.GetFields(ctx, &GetFieldsRequest{
			TenantID:   tenantID,
			FieldPaths: fieldPaths,
		})
		if err != nil {
			return nil, err
		}
		result := make(map[string]string, len(resp.Values))
		for _, v := range resp.Values {
			result[v.FieldPath] = v.Value.String()
		}
		return result, nil
	})
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
	switch tv.Kind() {
	case KindString, KindURL, KindJSON:
		return tv.str, nil
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
	if tv.Kind() == KindInteger {
		return tv.i, nil
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
	if tv.Kind() == KindNumber {
		return tv.num, nil
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
	if tv.Kind() == KindBool {
		return tv.b, nil
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
	if tv.Kind() == KindTime {
		return tv.t, nil
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
	if tv.Kind() == KindDuration {
		return tv.d, nil
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
	s := tv.String()
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
	if tv.Kind() == KindInteger {
		v := tv.i
		return &v, nil
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
	if tv.Kind() == KindBool {
		v := tv.b
		return &v, nil
	}
	return nil, ErrTypeMismatch
}

// --- Internal helpers ---

func (c *Client) getTypedValue(ctx context.Context, tenantID, fieldPath string) (*TypedValue, error) {
	return retry(ctx, c, func(ctx context.Context) (*TypedValue, error) {
		resp, err := c.transport.GetField(ctx, &GetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
		})
		if err != nil {
			return nil, err
		}
		return resp.Value, nil
	})
}

func configToMap(resp *GetConfigResponse) map[string]string {
	if resp == nil || len(resp.Values) == 0 {
		return nil
	}
	m := make(map[string]string, len(resp.Values))
	for _, v := range resp.Values {
		m[v.FieldPath] = v.Value.String()
	}
	return m
}
