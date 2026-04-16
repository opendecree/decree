package adminclient

import (
	"context"
	"time"
)

// AuditFilter configures which audit entries to retrieve.
type AuditFilter func(*QueryWriteLogRequest)

// WithAuditTenant filters audit entries by tenant ID.
func WithAuditTenant(tenantID string) AuditFilter {
	return func(r *QueryWriteLogRequest) { r.TenantID = &tenantID }
}

// WithAuditActor filters audit entries by actor (JWT subject).
func WithAuditActor(actor string) AuditFilter {
	return func(r *QueryWriteLogRequest) { r.Actor = &actor }
}

// WithAuditField filters audit entries by field path.
func WithAuditField(fieldPath string) AuditFilter {
	return func(r *QueryWriteLogRequest) { r.FieldPath = &fieldPath }
}

// WithAuditTimeRange filters audit entries by creation time range.
// Either start or end may be nil for an open-ended range.
func WithAuditTimeRange(start, end *time.Time) AuditFilter {
	return func(r *QueryWriteLogRequest) {
		r.StartTime = start
		r.EndTime = end
	}
}

// QueryWriteLog searches the audit log for config change events.
// Filters are optional — omit all filters to retrieve all entries.
// Auto-paginates through all results.
func (c *Client) QueryWriteLog(ctx context.Context, filters ...AuditFilter) ([]*AuditEntry, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	var all []*AuditEntry
	pageToken := ""
	for {
		req := &QueryWriteLogRequest{
			PageSize:  100,
			PageToken: pageToken,
		}
		for _, f := range filters {
			f(req)
		}
		resp, err := c.audit.QueryWriteLog(ctx, req)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Entries...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return all, nil
}

// GetFieldUsage returns aggregated read statistics for a specific field.
// Start and end times are optional — pass nil for open-ended ranges.
func (c *Client) GetFieldUsage(ctx context.Context, tenantID, fieldPath string, start, end *time.Time) (*UsageStats, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.audit.GetFieldUsage(ctx, tenantID, fieldPath, start, end)
}

// GetTenantUsage returns aggregated read statistics for all fields of a tenant.
// Start and end times are optional — pass nil for open-ended ranges.
func (c *Client) GetTenantUsage(ctx context.Context, tenantID string, start, end *time.Time) ([]*UsageStats, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.audit.GetTenantUsage(ctx, tenantID, start, end)
}

// GetUnusedFields returns field paths that have not been read since the given time.
// Useful for identifying configuration fields that may be safe to deprecate.
func (c *Client) GetUnusedFields(ctx context.Context, tenantID string, since time.Time) ([]string, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.audit.GetUnusedFields(ctx, tenantID, since)
}
