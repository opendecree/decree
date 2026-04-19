package configclient

import (
	"context"
	"time"
)

// Set writes a single configuration value as a string.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is locked.
func (c *Client) Set(ctx context.Context, tenantID, fieldPath, value string) error {
	return retryDo(ctx, c, func(ctx context.Context) error {
		_, err := c.transport.SetField(ctx, &SetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
			Value:     StringVal(value),
		})
		return err
	})
}

// SetTyped writes a single typed configuration value.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is locked.
func (c *Client) SetTyped(ctx context.Context, tenantID, fieldPath string, value *TypedValue) error {
	return retryDo(ctx, c, func(ctx context.Context) error {
		_, err := c.transport.SetField(ctx, &SetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
			Value:     value,
		})
		return err
	})
}

// SetNull sets a configuration field to null.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is locked.
func (c *Client) SetNull(ctx context.Context, tenantID, fieldPath string) error {
	return retryDo(ctx, c, func(ctx context.Context) error {
		_, err := c.transport.SetField(ctx, &SetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
		})
		return err
	})
}

// SetMany writes multiple configuration values atomically in a single version.
// The description is optional — pass an empty string to omit it.
// Returns [ErrLocked] if any of the fields are locked.
func (c *Client) SetMany(ctx context.Context, tenantID string, values map[string]string, description string) error {
	return retryDo(ctx, c, func(ctx context.Context) error {
		updates := make([]FieldUpdate, 0, len(values))
		for path, val := range values {
			v := StringVal(val)
			updates = append(updates, FieldUpdate{
				FieldPath: path,
				Value:     v,
			})
		}
		_, err := c.transport.SetFields(ctx, &SetFieldsRequest{
			TenantID:    tenantID,
			Updates:     updates,
			Description: description,
		})
		return err
	})
}

// SetManyTyped writes multiple typed configuration values atomically in a single
// version. The description is optional — pass an empty string to omit it.
// Returns [ErrLocked] if any of the fields are locked.
func (c *Client) SetManyTyped(ctx context.Context, tenantID string, values map[string]*TypedValue, description string) error {
	return retryDo(ctx, c, func(ctx context.Context) error {
		updates := make([]FieldUpdate, 0, len(values))
		for path, v := range values {
			updates = append(updates, FieldUpdate{
				FieldPath: path,
				Value:     v,
			})
		}
		_, err := c.transport.SetFields(ctx, &SetFieldsRequest{
			TenantID:    tenantID,
			Updates:     updates,
			Description: description,
		})
		return err
	})
}

// LockedValue holds a field's current value and checksum for optimistic concurrency.
// Use [Client.GetForUpdate] to obtain one, then call [LockedValue.Set] to write
// a new value only if the field hasn't been modified since the read.
type LockedValue struct {
	// FieldPath is the dot-separated field path.
	FieldPath string
	// Value is the current value at the time of the read.
	Value string
	// Checksum is the hash of the value, used for compare-and-swap.
	Checksum string

	tenantID string
}

// GetForUpdate reads a field's current value along with its checksum.
// The returned [LockedValue] can be used to perform a conditional write via
// [LockedValue.Set], which will fail with [ErrChecksumMismatch] if the value
// was modified between the read and the write.
func (c *Client) GetForUpdate(ctx context.Context, tenantID, fieldPath string) (*LockedValue, error) {
	return retry(ctx, c, func(ctx context.Context) (*LockedValue, error) {
		resp, err := c.transport.GetField(ctx, &GetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
		})
		if err != nil {
			return nil, err
		}
		return &LockedValue{
			FieldPath: fieldPath,
			Value:     resp.Value.String(),
			Checksum:  resp.Checksum,
			tenantID:  tenantID,
		}, nil
	})
}

// Set writes a new value for this field, but only if the value has not been
// modified since the [LockedValue] was obtained via [Client.GetForUpdate].
// Returns [ErrChecksumMismatch] if the value was changed by another writer.
func (lv *LockedValue) Set(ctx context.Context, client *Client, newValue string) error {
	return retryDo(ctx, client, func(ctx context.Context) error {
		_, err := client.transport.SetField(ctx, &SetFieldRequest{
			TenantID:         lv.tenantID,
			FieldPath:        lv.FieldPath,
			Value:            StringVal(newValue),
			ExpectedChecksum: &lv.Checksum,
		})
		return err
	})
}

// Update performs an atomic read-modify-write on a single field.
// It reads the current value and checksum, calls updateFn with the current value,
// and writes the result back with the checksum for optimistic concurrency.
//
// Returns [ErrChecksumMismatch] if the value was modified between the read and write.
// Returns [ErrNotFound] if the field has no value set.
func (c *Client) Update(ctx context.Context, tenantID, fieldPath string, updateFn func(current string) (string, error)) error {
	lv, err := c.GetForUpdate(ctx, tenantID, fieldPath)
	if err != nil {
		return err
	}
	newValue, err := updateFn(lv.Value)
	if err != nil {
		return err
	}
	return lv.Set(ctx, c, newValue)
}

// --- Type-specific setters ---

// SetInt writes an integer configuration value.
func (c *Client) SetInt(ctx context.Context, tenantID, fieldPath string, value int64) error {
	return c.setTyped(ctx, tenantID, fieldPath, IntVal(value))
}

// SetFloat writes a floating-point configuration value.
func (c *Client) SetFloat(ctx context.Context, tenantID, fieldPath string, value float64) error {
	return c.setTyped(ctx, tenantID, fieldPath, FloatVal(value))
}

// SetBool writes a boolean configuration value.
func (c *Client) SetBool(ctx context.Context, tenantID, fieldPath string, value bool) error {
	return c.setTyped(ctx, tenantID, fieldPath, BoolVal(value))
}

// SetTime writes a timestamp configuration value.
func (c *Client) SetTime(ctx context.Context, tenantID, fieldPath string, value time.Time) error {
	return c.setTyped(ctx, tenantID, fieldPath, TimeVal(value))
}

// SetDuration writes a duration configuration value.
func (c *Client) SetDuration(ctx context.Context, tenantID, fieldPath string, value time.Duration) error {
	return c.setTyped(ctx, tenantID, fieldPath, DurationVal(value))
}

func (c *Client) setTyped(ctx context.Context, tenantID, fieldPath string, value *TypedValue) error {
	return retryDo(ctx, c, func(ctx context.Context) error {
		_, err := c.transport.SetField(ctx, &SetFieldRequest{
			TenantID:  tenantID,
			FieldPath: fieldPath,
			Value:     value,
		})
		return err
	})
}
