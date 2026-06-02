package adminclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
		token := pageToken
		resp, err := retry(ctx, c, func(ctx context.Context) (*QueryWriteLogResponse, error) {
			req := &QueryWriteLogRequest{
				PageSize:  100,
				PageToken: token,
			}
			for _, f := range filters {
				f(req)
			}
			return c.audit.QueryWriteLog(ctx, req)
		})
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
// Tail truncation (removal of the genesis entry or oldest entries) is detected
// by asserting that the oldest entry has previous_hash == "". Head truncation
// (removal of the newest entries) cannot be detected without a server-provided
// authoritative head hash; VerifyChain does not attempt to detect it.
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
		resp, err := retry(ctx, c, func(ctx context.Context) (*QueryWriteLogResponse, error) {
			return c.audit.QueryWriteLog(ctx, req)
		})
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
			want := computeClientHash(prev, entry)
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

	// Genesis check: the oldest entry must have previous_hash == "".
	// A non-empty previous_hash means the chain is missing its beginning (tail truncation).
	if total > 0 {
		oldest := pages[len(pages)-1][len(pages[len(pages)-1])-1]
		if oldest.PreviousHash != "" {
			result.OK = false
			result.Breaks = append(result.Breaks, VerifyChainBreak{
				EntryID:  oldest.ID,
				Position: 0,
				Got:      oldest.PreviousHash,
				Want:     "",
				Reason:   "chain is truncated: oldest entry has non-empty previous_hash",
			})
		}
	}

	return result, nil
}

func computeClientHash(previousHash string, e *AuditEntry) string {
	h := sha256.New()
	if e.ChainEpoch == 0 {
		// Epoch 0: structural fields only (legacy, backward compat).
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d",
			previousHash, e.ID, e.TenantID, e.Actor, e.Action, e.ObjectKind, e.CreatedAt.UnixNano())
		return hex.EncodeToString(h.Sum(nil))
	}
	// Epoch 1+: structural fields followed by payload fields.
	// Mirrors internal/audit.ComputeEntryHash epoch-1 logic exactly.
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00",
		previousHash, e.ID, e.TenantID, e.Actor, e.Action, e.ObjectKind, e.CreatedAt.UnixNano())
	writeNullableStr(h, e.FieldPath)
	writeNullableStr(h, e.OldValue)
	writeNullableStr(h, e.NewValue)
	writeNullableI32(h, e.ConfigVersion)
	if len(e.Metadata) == 0 {
		h.Write([]byte{0x00})
	} else {
		// Metadata is hashed as sorted JSON to produce a deterministic encoding
		// that matches the server's encoding of the JSONB bytes.
		h.Write([]byte{0x01})
		b, _ := marshalSortedJSON(e.Metadata)
		fmt.Fprint(h, hex.EncodeToString(b))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeNullableStr(h interface{ Write([]byte) (int, error) }, s string) {
	if s == "" {
		h.Write([]byte{0x00})
	} else {
		h.Write([]byte{0x01})
		fmt.Fprintf(h, "%s\x00", s)
	}
}

func writeNullableI32(h interface{ Write([]byte) (int, error) }, v *int32) {
	if v == nil {
		h.Write([]byte{0x00})
	} else {
		fmt.Fprintf(h, "\x01%d\x00", *v)
	}
}

func marshalSortedJSON(m map[string]string) ([]byte, error) {
	// encoding/json marshals map keys in sorted order by default.
	return json.Marshal(m)
}
