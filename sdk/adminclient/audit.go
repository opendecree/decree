package adminclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
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

// VerifyChain fetches all audit entries for tenantID (oldest-first) and
// recomputes each entry_hash, reporting any tampered positions.
// An empty tenantID verifies the global (schema-level) chain.
//
// Note: entry_hash and previous_hash fields require the server to be running
// with migration 002_audit_tamper_evident applied.
func (c *Client) VerifyChain(ctx context.Context, tenantID string) (VerifyChainResult, error) {
	if c.audit == nil {
		return VerifyChainResult{}, ErrServiceNotConfigured
	}

	var filters []AuditFilter
	if tenantID != "" {
		filters = append(filters, WithAuditTenant(tenantID))
	}
	entries, err := c.QueryWriteLog(ctx, filters...)
	if err != nil {
		return VerifyChainResult{}, fmt.Errorf("fetch entries: %w", err)
	}

	// QueryWriteLog returns newest-first; sort oldest-first for chain walk.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})

	result := VerifyChainResult{TenantID: tenantID, Total: len(entries)}
	prev := ""
	for i, e := range entries {
		want := computeClientHash(prev, e.ID, e.TenantID, e.Actor, e.Action, e.ObjectKind, e.CreatedAt)
		if e.EntryHash != want {
			result.Breaks = append(result.Breaks, VerifyChainBreak{
				EntryID:  e.ID,
				Position: i,
				Got:      e.EntryHash,
				Want:     want,
			})
		}
		prev = e.EntryHash
	}
	result.OK = len(result.Breaks) == 0
	return result, nil
}

func computeClientHash(previousHash, id, tenantID, actor, action, objectKind string, createdAt time.Time) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d",
		previousHash, id, tenantID, actor, action, objectKind, createdAt.UnixNano())
	return hex.EncodeToString(h.Sum(nil))
}
