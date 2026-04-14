package configclient

import (
	"context"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// Set writes a single configuration value as a string.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is locked.
func (c *Client) Set(ctx context.Context, tenantID, fieldPath, value string) error {
	_, err := c.rpc.SetField(c.withAuth(ctx), &pb.SetFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
		Value:     StringValue(value),
	})
	return mapError(err)
}

// SetTyped writes a single typed configuration value.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is locked.
func (c *Client) SetTyped(ctx context.Context, tenantID, fieldPath string, value *pb.TypedValue) error {
	_, err := c.rpc.SetField(c.withAuth(ctx), &pb.SetFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
		Value:     value,
	})
	return mapError(err)
}

// SetNull sets a configuration field to null.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is locked.
func (c *Client) SetNull(ctx context.Context, tenantID, fieldPath string) error {
	_, err := c.rpc.SetField(c.withAuth(ctx), &pb.SetFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
		// Value is nil — sets the field to null.
	})
	return mapError(err)
}

// SetMany writes multiple configuration values atomically in a single version.
// The description is optional — pass an empty string to omit it.
// Returns [ErrLocked] if any of the fields are locked.
func (c *Client) SetMany(ctx context.Context, tenantID string, values map[string]string, description string) error {
	updates := make([]*pb.FieldUpdate, 0, len(values))
	for path, val := range values {
		updates = append(updates, &pb.FieldUpdate{
			FieldPath: path,
			Value:     StringValue(val),
		})
	}
	req := &pb.SetFieldsRequest{
		TenantId: tenantID,
		Updates:  updates,
	}
	if description != "" {
		req.Description = &description
	}
	_, err := c.rpc.SetFields(c.withAuth(ctx), req)
	return mapError(err)
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
//
// This is useful when you need to coordinate updates across multiple fields
// or when the update logic is complex. For simple single-field updates,
// consider using [Client.Update] instead.
func (c *Client) GetForUpdate(ctx context.Context, tenantID, fieldPath string) (*LockedValue, error) {
	resp, err := c.rpc.GetField(c.withAuth(ctx), &pb.GetFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &LockedValue{
		FieldPath: fieldPath,
		Value:     typedValueToString(resp.Value.Value),
		Checksum:  resp.Value.Checksum,
		tenantID:  tenantID,
	}, nil
}

// Set writes a new value for this field, but only if the value has not been
// modified since the [LockedValue] was obtained via [Client.GetForUpdate].
// Returns [ErrChecksumMismatch] if the value was changed by another writer.
func (lv *LockedValue) Set(ctx context.Context, client *Client, newValue string) error {
	_, err := client.rpc.SetField(client.withAuth(ctx), &pb.SetFieldRequest{
		TenantId:         lv.tenantID,
		FieldPath:        lv.FieldPath,
		Value:            StringValue(newValue),
		ExpectedChecksum: &lv.Checksum,
	})
	return mapError(err)
}

// Update performs an atomic read-modify-write on a single field.
// It reads the current value and checksum, calls updateFn with the current value,
// and writes the result back with the checksum for optimistic concurrency.
//
// Returns [ErrChecksumMismatch] if the value was modified between the read and write.
// Returns [ErrNotFound] if the field has no value set.
//
// Example:
//
//	err := client.Update(ctx, tenantID, "counter", func(current string) (string, error) {
//	    n, _ := strconv.Atoi(current)
//	    return strconv.Itoa(n + 1), nil
//	})
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
