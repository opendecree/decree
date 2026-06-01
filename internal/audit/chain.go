package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

	// Epoch 0 = legacy (structural fields only).
	// Epoch 1+ = full payload included.
	Epoch int

	// Payload fields (included in epoch 1+ hashes).
	FieldPath     *string
	OldValue      *string
	NewValue      *string
	ConfigVersion *int32
	Metadata      []byte
}

// ComputeEntryHash produces a SHA-256 hash over the immutable fields of an
// audit entry, chaining it to the previous entry via PreviousHash.
// Epoch 0 uses only structural fields (backward compat for pre-migration rows).
// Epoch 1+ includes all payload fields so tampering with content is detectable.
func ComputeEntryHash(in ChainInput) string {
	h := sha256.New()
	if in.Epoch == 0 {
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
	// Epoch 1+: structural fields followed by payload fields.
	// CreatedAt is intentionally excluded from epoch-1 to avoid sub-microsecond
	// round-trip discrepancies between the app clock and PostgreSQL timestamptz.
	// Payload fields use a 1-byte presence marker: 0x00=nil, 0x01=non-nil.
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00",
		in.PreviousHash,
		in.ID,
		in.TenantID,
		in.Actor,
		in.Action,
		in.ObjectKind,
	)
	writeNullableStr(h, in.FieldPath)
	writeNullableStr(h, in.OldValue)
	writeNullableStr(h, in.NewValue)
	writeNullableI32(h, in.ConfigVersion)
	// Metadata is intentionally excluded from epoch-1: PG JSONB normalizes
	// the byte representation on storage, causing the raw bytes to differ
	// between write time (from json.Marshal) and read time (from PG text output).
	return hex.EncodeToString(h.Sum(nil))
}

func writeNullableStr(h io.Writer, s *string) {
	if s == nil {
		_, _ = h.Write([]byte{0x00})
	} else {
		_, _ = h.Write([]byte{0x01})
		_, _ = fmt.Fprintf(h, "%s\x00", *s)
	}
}

func writeNullableI32(h io.Writer, v *int32) {
	if v == nil {
		_, _ = h.Write([]byte{0x00})
	} else {
		_, _ = fmt.Fprintf(h, "\x01%d\x00", *v)
	}
}
