package adminclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	return retry(ctx, c, func(ctx context.Context) ([]*AuditEntry, error) {
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
	})
}

// AuditIterator is a streaming handle returned by [Client.QueryWriteLogIter].
// Entries are sent to C page by page. C is closed when all entries have been sent
// or a transport error occurs. After C is closed, read Err to check for errors.
//
// Memory usage is bounded to one page of entries at a time.
type AuditIterator struct {
	// C receives audit entries in server-returned order (newest first by default).
	C chan *AuditEntry
	// Err receives at most one error, then is closed. Read after C is closed.
	Err chan error
}

// QueryWriteLogIter streams audit log entries page by page without accumulating
// all results in memory. Entries are sent to the returned [AuditIterator].C channel.
// Cancel ctx to stop early.
//
// Usage:
//
//	it := client.QueryWriteLogIter(ctx, filters...)
//	for e := range it.C {
//	    // process e
//	}
//	if err := <-it.Err; err != nil {
//	    // handle err
//	}
func (c *Client) QueryWriteLogIter(ctx context.Context, filters ...AuditFilter) *AuditIterator {
	ch := make(chan *AuditEntry, 100)
	errc := make(chan error, 1)
	it := &AuditIterator{C: ch, Err: errc}

	if c.audit == nil {
		close(ch)
		errc <- ErrServiceNotConfigured
		close(errc)
		return it
	}

	go func() {
		defer close(ch)
		defer close(errc)
		pageToken := ""
		for {
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			default:
			}
			req := &QueryWriteLogRequest{
				PageSize:  100,
				PageToken: pageToken,
			}
			for _, f := range filters {
				f(req)
			}
			resp, err := c.audit.QueryWriteLog(ctx, req)
			if err != nil {
				errc <- err
				return
			}
			for _, e := range resp.Entries {
				select {
				case ch <- e:
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				}
			}
			if resp.NextPageToken == "" {
				return
			}
			pageToken = resp.NextPageToken
		}
	}()
	return it
}

// GetFieldUsage returns aggregated read statistics for a specific field.
// Start and end times are optional — pass nil for open-ended ranges.
func (c *Client) GetFieldUsage(ctx context.Context, tenantID, fieldPath string, start, end *time.Time) (*UsageStats, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	return retry(ctx, c, func(ctx context.Context) (*UsageStats, error) {
		return c.audit.GetFieldUsage(ctx, tenantID, fieldPath, start, end)
	})
}

// GetTenantUsage returns aggregated read statistics for all fields of a tenant.
// Start and end times are optional — pass nil for open-ended ranges.
func (c *Client) GetTenantUsage(ctx context.Context, tenantID string, start, end *time.Time) ([]*UsageStats, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	return retry(ctx, c, func(ctx context.Context) ([]*UsageStats, error) {
		return c.audit.GetTenantUsage(ctx, tenantID, start, end)
	})
}

// GetUnusedFields returns field paths that have not been read since the given time.
// Useful for identifying configuration fields that may be safe to deprecate.
func (c *Client) GetUnusedFields(ctx context.Context, tenantID string, since time.Time) ([]string, error) {
	if c.audit == nil {
		return nil, ErrServiceNotConfigured
	}
	return retry(ctx, c, func(ctx context.Context) ([]string, error) {
		return c.audit.GetUnusedFields(ctx, tenantID, since)
	})
}

// VerifyChain fetches all audit entries for tenantID and recomputes each
// entry_hash, reporting any tampered positions.
// An empty tenantID verifies the global (schema-level) chain.
//
// Entries are fetched page by page and processed with a running hash to keep
// memory bounded to O(pages) rather than O(all entries). The server returns
// pages newest-first; VerifyChain processes them in reverse to walk oldest-first
// without an in-memory sort.
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

	// Collect pages without accumulating a single flat slice.
	// Server returns pages newest-first; entries within each page are also newest-first.
	var pages [][]*AuditEntry
	total := 0
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
			return VerifyChainResult{}, fmt.Errorf("fetch entries: %w", err)
		}
		if len(resp.Entries) > 0 {
			pages = append(pages, resp.Entries)
			total += len(resp.Entries)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	// Walk pages in reverse (last page = oldest entries) and entries within each
	// page in reverse (last entry in page = oldest), maintaining a running hash.
	// This avoids an in-memory sort while preserving chronological order.
	result := VerifyChainResult{TenantID: tenantID, Total: total}
	prev := ""
	pos := 0
	for p := len(pages) - 1; p >= 0; p-- {
		page := pages[p]
		for e := len(page) - 1; e >= 0; e-- {
			entry := page[e]
			var fieldPath, oldValue, newValue *string
			if entry.FieldPath != "" {
				fp := entry.FieldPath
				fieldPath = &fp
			}
			if entry.OldValue != "" {
				ov := entry.OldValue
				oldValue = &ov
			}
			if entry.NewValue != "" {
				nv := entry.NewValue
				newValue = &nv
			}
			want := computeClientHash(entry.ChainEpoch, prev, entry.ID, entry.TenantID, entry.Actor, entry.Action, entry.ObjectKind, entry.CreatedAt, fieldPath, oldValue, newValue, entry.ConfigVersion, entry.Metadata)
			if entry.EntryHash != want {
				result.Breaks = append(result.Breaks, VerifyChainBreak{
					EntryID:  entry.ID,
					Position: pos,
					Got:      entry.EntryHash,
					Want:     want,
				})
			}
			prev = entry.EntryHash
			pos++
		}
	}
	result.OK = len(result.Breaks) == 0
	return result, nil
}

// computeClientHash replicates the server's ComputeEntryHash logic for the
// given epoch. Epoch 0 hashes only structural fields (backward compat).
// Epoch 1 includes all payload fields so content tampering is detectable.
func computeClientHash(epoch int32, previousHash, id, tenantID, actor, action, objectKind string, createdAt time.Time, fieldPath, oldValue, newValue *string, configVersion *int32, metadata []byte) string {
	h := sha256.New()
	if epoch == 0 {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d",
			previousHash, id, tenantID, actor, action, objectKind, createdAt.UnixNano())
		return hex.EncodeToString(h.Sum(nil))
	}
	// Epoch 1+: structural fields followed by payload fields.
	// Payload fields use a 1-byte presence marker: 0x00=nil, 0x01=non-nil.
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00",
		previousHash, id, tenantID, actor, action, objectKind, createdAt.UnixNano())
	writeNullableClientStr(h, fieldPath)
	writeNullableClientStr(h, oldValue)
	writeNullableClientStr(h, newValue)
	writeNullableClientI32(h, configVersion)
	if len(metadata) == 0 {
		_, _ = h.Write([]byte{0x00})
	} else {
		_, _ = h.Write([]byte{0x01})
		fmt.Fprint(h, hex.EncodeToString(metadata))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeNullableClientStr(h io.Writer, s *string) {
	if s == nil {
		_, _ = h.Write([]byte{0x00})
	} else {
		_, _ = h.Write([]byte{0x01})
		fmt.Fprintf(h, "%s\x00", *s)
	}
}

func writeNullableClientI32(h io.Writer, v *int32) {
	if v == nil {
		_, _ = h.Write([]byte{0x00})
	} else {
		fmt.Fprintf(h, "\x01%d\x00", *v)
	}
}
