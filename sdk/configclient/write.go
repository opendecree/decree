package configclient

import (
	"context"
	"time"
)

// WriteOption configures a single write operation.
//
// Write operations (Set, SetMany, etc.) do not retry by default because
// they are not idempotent without an idempotency key — a retry after a
// transient error could apply the write twice. Use [WithIdempotencyKey]
// to opt in to safe retry.
type WriteOption func(*writeOptions)

type writeOptions struct {
	idempotencyKey    string
	description       string
	valueDescription  string
	expectedChecksum  string
	valueDescriptions map[string]string // per-field value descriptions for batch writes
	fieldChecksums    map[string]string // per-field expected checksums for batch writes
}

func applyWriteOptions(opts []WriteOption) writeOptions {
	var wo writeOptions
	for _, o := range opts {
		o(&wo)
	}
	return wo
}

// WithIdempotencyKey attaches an idempotency key to a write operation.
// When set, the server deduplicates writes with the same key within a
// 24-hour window, making the write safe to retry on transient errors.
// If the client was created with [WithRetry], retry is enabled for this call.
//
// The key must be unique per logical write; use a UUID or similarly
// collision-resistant value. Keys are scoped to the tenant.
func WithIdempotencyKey(key string) WriteOption {
	return func(o *writeOptions) { o.idempotencyKey = key }
}

// WithDescription sets a version-level description explaining why the change was made.
func WithDescription(desc string) WriteOption {
	return func(o *writeOptions) { o.description = desc }
}

// WithValueDescription sets a field-level description explaining what the value means.
// Retrievable via include_description in read requests.
func WithValueDescription(desc string) WriteOption {
	return func(o *writeOptions) { o.valueDescription = desc }
}

// WithExpectedChecksum makes the write conditional: it succeeds only if the
// field's current checksum matches the provided value. Returns [ErrChecksumMismatch]
// on conflict. Use [Client.GetForUpdate] to obtain the current checksum.
func WithExpectedChecksum(checksum string) WriteOption {
	return func(o *writeOptions) { o.expectedChecksum = checksum }
}

// WithValueDescriptions sets per-field value descriptions for batch write operations
// ([Client.SetMany], [Client.SetManyTyped]). The map key is the field path.
func WithValueDescriptions(descs map[string]string) WriteOption {
	return func(o *writeOptions) { o.valueDescriptions = descs }
}

// WithFieldChecksums sets per-field expected checksums for batch write operations
// ([Client.SetMany], [Client.SetManyTyped]). The write fails with [ErrChecksumMismatch]
// if any field's current checksum does not match. The map key is the field path.
func WithFieldChecksums(checksums map[string]string) WriteOption {
	return func(o *writeOptions) { o.fieldChecksums = checksums }
}

// doWrite executes a write operation. Without an idempotency key or expected
// checksum, the call is made exactly once. With either (both make the write
// idempotent) and retry enabled on the client, transient errors trigger retry.
func doWrite(ctx context.Context, c *Client, wo writeOptions, fn func(ctx context.Context) error) error {
	if (wo.idempotencyKey != "" || wo.expectedChecksum != "") && c.opts.retryEnabled {
		return retryDo(ctx, c, fn)
	}
	return fn(ctx)
}

// Set writes a single configuration value as a string.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is administratively locked; [ErrPermissionDenied] if the caller lacks access.
//
// By default, Set does not retry on transient errors. Use [WithIdempotencyKey]
// to opt in to safe retry with server-side deduplication.
func (c *Client) Set(ctx context.Context, tenantID, fieldPath, value string, opts ...WriteOption) error {
	wo := applyWriteOptions(opts)
	return doWrite(ctx, c, wo, func(ctx context.Context) error {
		req := &SetFieldRequest{
			TenantID:         tenantID,
			FieldPath:        fieldPath,
			Value:            StringVal(value),
			Description:      wo.description,
			ValueDescription: wo.valueDescription,
			IdempotencyKey:   wo.idempotencyKey,
		}
		if wo.expectedChecksum != "" {
			req.ExpectedChecksum = &wo.expectedChecksum
		}
		_, err := c.transport.SetField(ctx, req)
		return err
	})
}

// SetTyped writes a single typed configuration value.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is administratively locked; [ErrPermissionDenied] if the caller lacks access.
//
// By default, SetTyped does not retry on transient errors. Use [WithIdempotencyKey]
// to opt in to safe retry with server-side deduplication.
func (c *Client) SetTyped(ctx context.Context, tenantID, fieldPath string, value *TypedValue, opts ...WriteOption) error {
	wo := applyWriteOptions(opts)
	return doWrite(ctx, c, wo, func(ctx context.Context) error {
		req := &SetFieldRequest{
			TenantID:         tenantID,
			FieldPath:        fieldPath,
			Value:            value,
			Description:      wo.description,
			ValueDescription: wo.valueDescription,
			IdempotencyKey:   wo.idempotencyKey,
		}
		if wo.expectedChecksum != "" {
			req.ExpectedChecksum = &wo.expectedChecksum
		}
		_, err := c.transport.SetField(ctx, req)
		return err
	})
}

// SetNull sets a configuration field to null.
// Creates a new config version atomically.
// Returns [ErrLocked] if the field is administratively locked; [ErrPermissionDenied] if the caller lacks access.
//
// By default, SetNull does not retry on transient errors. Use [WithIdempotencyKey]
// to opt in to safe retry with server-side deduplication.
func (c *Client) SetNull(ctx context.Context, tenantID, fieldPath string, opts ...WriteOption) error {
	wo := applyWriteOptions(opts)
	return doWrite(ctx, c, wo, func(ctx context.Context) error {
		req := &SetFieldRequest{
			TenantID:       tenantID,
			FieldPath:      fieldPath,
			Description:    wo.description,
			IdempotencyKey: wo.idempotencyKey,
		}
		if wo.expectedChecksum != "" {
			req.ExpectedChecksum = &wo.expectedChecksum
		}
		_, err := c.transport.SetField(ctx, req)
		return err
	})
}

// SetMany writes multiple configuration values atomically in a single version.
// The description is optional — pass an empty string to omit it.
// Returns [ErrLocked] if any field is administratively locked; [ErrPermissionDenied] if the caller lacks access.
//
// By default, SetMany does not retry on transient errors. Use [WithIdempotencyKey]
// to opt in to safe retry with server-side deduplication.
func (c *Client) SetMany(ctx context.Context, tenantID string, values map[string]string, description string, opts ...WriteOption) error {
	wo := applyWriteOptions(opts)
	return doWrite(ctx, c, wo, func(ctx context.Context) error {
		updates := make([]FieldUpdate, 0, len(values))
		for path, val := range values {
			u := FieldUpdate{
				FieldPath:        path,
				Value:            StringVal(val),
				ValueDescription: wo.valueDescriptions[path],
			}
			if chk := wo.fieldChecksums[path]; chk != "" {
				u.ExpectedChecksum = &chk
			}
			updates = append(updates, u)
		}
		_, err := c.transport.SetFields(ctx, &SetFieldsRequest{
			TenantID:       tenantID,
			Updates:        updates,
			Description:    description,
			IdempotencyKey: wo.idempotencyKey,
		})
		return err
	})
}

// SetManyTyped writes multiple typed configuration values atomically in a single
// version. The description is optional — pass an empty string to omit it.
// Returns [ErrLocked] if any field is administratively locked; [ErrPermissionDenied] if the caller lacks access.
//
// By default, SetManyTyped does not retry on transient errors. Use [WithIdempotencyKey]
// to opt in to safe retry with server-side deduplication.
func (c *Client) SetManyTyped(ctx context.Context, tenantID string, values map[string]*TypedValue, description string, opts ...WriteOption) error {
	wo := applyWriteOptions(opts)
	return doWrite(ctx, c, wo, func(ctx context.Context) error {
		updates := make([]FieldUpdate, 0, len(values))
		for path, v := range values {
			u := FieldUpdate{
				FieldPath:        path,
				Value:            v,
				ValueDescription: wo.valueDescriptions[path],
			}
			if chk := wo.fieldChecksums[path]; chk != "" {
				u.ExpectedChecksum = &chk
			}
			updates = append(updates, u)
		}
		_, err := c.transport.SetFields(ctx, &SetFieldsRequest{
			TenantID:       tenantID,
			Updates:        updates,
			Description:    description,
			IdempotencyKey: wo.idempotencyKey,
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
	client   *Client
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
			client:    c,
		}, nil
	})
}

// Set writes a new value for this field, but only if the value has not been
// modified since the [LockedValue] was obtained via [Client.GetForUpdate].
// Returns [ErrChecksumMismatch] if the value was changed by another writer.
//
// Because ExpectedChecksum makes this write idempotent, this method respects
// the client's retry configuration.
func (lv *LockedValue) Set(ctx context.Context, newValue string, opts ...WriteOption) error {
	wo := applyWriteOptions(opts)
	// LockedValue.Set is always safe to retry: the checksum acts as an implicit
	// idempotency guard. Use retryDo directly to preserve that guarantee.
	return retryDo(ctx, lv.client, func(ctx context.Context) error {
		req := &SetFieldRequest{
			TenantID:         lv.tenantID,
			FieldPath:        lv.FieldPath,
			Value:            StringVal(newValue),
			ExpectedChecksum: &lv.Checksum,
			Description:      wo.description,
			ValueDescription: wo.valueDescription,
			IdempotencyKey:   wo.idempotencyKey,
		}
		_, err := lv.client.transport.SetField(ctx, req)
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
	return lv.Set(ctx, newValue)
}

// --- Type-specific setters ---

// SetInt writes an integer configuration value.
func (c *Client) SetInt(ctx context.Context, tenantID, fieldPath string, value int64, opts ...WriteOption) error {
	return c.setTyped(ctx, tenantID, fieldPath, IntVal(value), opts...)
}

// SetFloat writes a floating-point configuration value.
func (c *Client) SetFloat(ctx context.Context, tenantID, fieldPath string, value float64, opts ...WriteOption) error {
	return c.setTyped(ctx, tenantID, fieldPath, FloatVal(value), opts...)
}

// SetBool writes a boolean configuration value.
func (c *Client) SetBool(ctx context.Context, tenantID, fieldPath string, value bool, opts ...WriteOption) error {
	return c.setTyped(ctx, tenantID, fieldPath, BoolVal(value), opts...)
}

// SetTime writes a timestamp configuration value.
func (c *Client) SetTime(ctx context.Context, tenantID, fieldPath string, value time.Time, opts ...WriteOption) error {
	return c.setTyped(ctx, tenantID, fieldPath, TimeVal(value), opts...)
}

// SetDuration writes a duration configuration value.
func (c *Client) SetDuration(ctx context.Context, tenantID, fieldPath string, value time.Duration, opts ...WriteOption) error {
	return c.setTyped(ctx, tenantID, fieldPath, DurationVal(value), opts...)
}

func (c *Client) setTyped(ctx context.Context, tenantID, fieldPath string, value *TypedValue, opts ...WriteOption) error {
	return c.SetTyped(ctx, tenantID, fieldPath, value, opts...)
}
