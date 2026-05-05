package audit

import (
	"context"
	"fmt"
)

// ChainBreak describes a single tampered or missing link in the audit chain.
type ChainBreak struct {
	EntryID  string
	Position int    // 0-based position in the tenant's ordered chain
	Got      string // entry_hash stored in the DB
	Want     string // hash we recomputed from the entry's fields
}

// VerifyResult is the outcome of a chain verification run.
type VerifyResult struct {
	TenantID string
	Total    int
	Breaks   []ChainBreak
	OK       bool
}

// VerifyChain fetches every audit entry for tenantID (ordered oldest-first) and
// recomputes each entry_hash, reporting any positions where stored ≠ computed.
// An empty tenantID verifies the global (schema-level) chain.
func VerifyChain(ctx context.Context, store Store, tenantID string) (VerifyResult, error) {
	entries, err := store.GetAuditWriteLogOrdered(ctx, tenantID)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("fetch audit chain: %w", err)
	}

	result := VerifyResult{TenantID: tenantID, Total: len(entries)}
	prev := ""
	for i, e := range entries {
		want := ComputeEntryHash(ChainInput{
			PreviousHash: prev,
			ID:           e.ID,
			TenantID:     e.TenantID,
			Actor:        e.Actor,
			Action:       e.Action,
			ObjectKind:   e.ObjectKind,
			CreatedAt:    e.CreatedAt,
		})
		if e.EntryHash != want {
			result.Breaks = append(result.Breaks, ChainBreak{
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
