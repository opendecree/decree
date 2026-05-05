package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ChainInput holds the fields that are hashed for a single audit entry.
type ChainInput struct {
	PreviousHash string
	ID           string
	TenantID     string // empty for global entries
	Actor        string
	Action       string
	ObjectKind   string
	CreatedAt    time.Time
}

// ComputeEntryHash produces a SHA-256 hash over the immutable fields of an
// audit entry, chaining it to the previous entry via PreviousHash.
func ComputeEntryHash(in ChainInput) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d",
		in.PreviousHash,
		in.ID,
		in.TenantID,
		in.Actor,
		in.Action,
		in.ObjectKind,
		in.CreatedAt.UnixNano(),
	)
	return hex.EncodeToString(h.Sum(nil))
}
