package adminclient

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQueryWriteLogIter_StreamsAllPages(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		switch req.PageToken {
		case "":
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "e1"}, {ID: "e2"}},
				NextPageToken: "p2",
			}, nil
		case "p2":
			return &QueryWriteLogResponse{
				Entries: []*AuditEntry{{ID: "e3"}},
			}, nil
		default:
			t.Fatalf("unexpected page token %q", req.PageToken)
			return nil, nil
		}
	}

	it := client.QueryWriteLogIter(context.Background())
	var got []string
	for e := range it.C {
		got = append(got, e.ID)
	}
	if err := <-it.Err; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3", len(got))
	}
	if got[0] != "e1" || got[1] != "e2" || got[2] != "e3" {
		t.Errorf("got IDs %v, want [e1 e2 e3]", got)
	}
}

func TestQueryWriteLogIter_TransportError(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	sentinel := errors.New("rpc failure")
	ma.queryWriteLogFn = func(_ context.Context, _ *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		return nil, sentinel
	}

	it := client.QueryWriteLogIter(context.Background())
	for range it.C {
	}
	if err := <-it.Err; !errors.Is(err, sentinel) {
		t.Errorf("got error %v, want sentinel", err)
	}
}

func TestQueryWriteLogIter_WithFilter(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	var capturedTenant *string
	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		capturedTenant = req.TenantID
		return &QueryWriteLogResponse{}, nil
	}

	it := client.QueryWriteLogIter(context.Background(), WithAuditTenant("t1"))
	for range it.C {
	}
	if err := <-it.Err; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTenant == nil || *capturedTenant != "t1" {
		t.Errorf("got TenantID %v, want t1", capturedTenant)
	}
}

func TestQueryWriteLogIter_ContextCancel(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	ctx, cancel := context.WithCancel(context.Background())

	// First page succeeds; cancel before goroutine asks for page 2.
	first := true
	ma.queryWriteLogFn = func(_ context.Context, _ *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if first {
			first = false
			cancel()
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "e1"}},
				NextPageToken: "p2",
			}, nil
		}
		// Should not be reached because context was cancelled.
		return &QueryWriteLogResponse{}, nil
	}

	it := client.QueryWriteLogIter(ctx)
	for range it.C {
	}
	if err := <-it.Err; !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want context.Canceled", err)
	}
}

// Golden hash values pre-computed by the reference implementation
// (internal/audit/chain.go ComputeEntryHash). These constants pin the exact
// output so that any change to the hash algorithm breaks the test instead of
// silently passing (circular tests would always agree with themselves).

const (
	// Epoch-0 chain: id-0, id-1, id-2 — tenant "t1", actor "actor", t0/t1/t2.
	goldenE0H0 = "d1aa4912744b449964c6e0f45c23382d516cde9eb890b3afcce3c1b5cf54c6fc"
	goldenE0H1 = "a473ca8a427fe513efc77e129b00b3fa35009c2fd023bec6c0a7b7ed39810f98"
	goldenE0H2 = "9277d017e924b4781212e6f4cceb1bc1300fb4b1f4e9004910d0fdd8a94dfa00"

	// Epoch-1 chain: id-0, id-1 — tenant "tenant-abc", actor "admin@example.com".
	// id-0: field_path="payments.fee", old="0.01", new="0.02", config_version=3, metadata={"source":"api"}
	// id-1: field_path="payments.fee", old="0.02", new="0.03", config_version=3, no metadata
	goldenE1H0 = "98958524d150f797252fba02d7b65b367f01ca1cfaab549248fcb646e461b85e"
	goldenE1H1 = "b43004ea033eaddc8273733656a825c1f4d02885952173c1cd001a063d8d9b58"
)

// TestComputeClientHash_GoldenValues verifies that computeClientHash produces
// the exact SHA-256 output expected by the server's ComputeEntryHash for both
// epoch 0 and epoch 1. If these fail, the client and server have diverged.
func TestComputeClientHash_GoldenValues(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)
	t2 := t0.Add(2 * time.Second)

	// Epoch 0: structural fields only, no payload.
	h0 := computeClientHash(0, "", "id-0", "t1", "actor", "set_field", "field", t0, nil, nil, nil, nil, nil)
	if h0 != goldenE0H0 {
		t.Errorf("epoch-0 h0: got %q, want %q", h0, goldenE0H0)
	}
	h1 := computeClientHash(0, goldenE0H0, "id-1", "t1", "actor", "set_field", "field", t1, nil, nil, nil, nil, nil)
	if h1 != goldenE0H1 {
		t.Errorf("epoch-0 h1: got %q, want %q", h1, goldenE0H1)
	}
	h2 := computeClientHash(0, goldenE0H1, "id-2", "t1", "actor", "set_field", "field", t2, nil, nil, nil, nil, nil)
	if h2 != goldenE0H2 {
		t.Errorf("epoch-0 h2: got %q, want %q", h2, goldenE0H2)
	}

	// Epoch 1: full payload included.
	fp := "payments.fee"
	ov0, nv0 := "0.01", "0.02"
	ov1, nv1 := "0.02", "0.03"
	cv := int32(3)
	meta := []byte(`{"source":"api"}`)

	e1h0 := computeClientHash(1, "", "id-0", "tenant-abc", "admin@example.com", "set_field", "field", t0, &fp, &ov0, &nv0, &cv, meta)
	if e1h0 != goldenE1H0 {
		t.Errorf("epoch-1 h0: got %q, want %q", e1h0, goldenE1H0)
	}
	e1h1 := computeClientHash(1, goldenE1H0, "id-1", "tenant-abc", "admin@example.com", "set_field", "field", t1, &fp, &ov1, &nv1, &cv, nil)
	if e1h1 != goldenE1H1 {
		t.Errorf("epoch-1 h1: got %q, want %q", e1h1, goldenE1H1)
	}
}

// TestVerifyChain_MultiPageIntact_Epoch0 tests the chunked streaming path where
// the server splits epoch-0 entries across multiple pages (newest first).
// Uses pre-computed golden hash values — not the function under test — so a
// wrong hash implementation cannot silently pass.
func TestVerifyChain_MultiPageIntact_Epoch0(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)
	t2 := t0.Add(2 * time.Second)

	// Server returns two pages, newest first.
	// Page 1: [id-2], Page 2: [id-1, id-0]
	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "id-2", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: goldenE0H2, CreatedAt: t2, ChainEpoch: 0}},
				NextPageToken: "p2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{
				{ID: "id-1", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: goldenE0H1, CreatedAt: t1, ChainEpoch: 0},
				{ID: "id-0", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: goldenE0H0, CreatedAt: t0, ChainEpoch: 0},
			},
		}, nil
	}

	result, err := client.VerifyChain(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK chain, got breaks: %+v", result.Breaks)
	}
	if result.Total != 3 {
		t.Errorf("got Total %d, want 3", result.Total)
	}
}

// TestVerifyChain_MultiPageTampered_Epoch0 verifies tampering is detected when
// epoch-0 entries span multiple pages. The tampered entry uses a hardcoded bad
// hash that differs from the golden value, confirming the verification rejects it.
func TestVerifyChain_MultiPageTampered_Epoch0(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	// id-1 has a tampered hash (not the correct chain continuation from goldenE0H0).
	badHash := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "id-1", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: badHash, CreatedAt: t1, ChainEpoch: 0}},
				NextPageToken: "p2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{{ID: "id-0", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: goldenE0H0, CreatedAt: t0, ChainEpoch: 0}},
		}, nil
	}

	result, err := client.VerifyChain(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OK {
		t.Error("expected chain not OK due to tampering")
	}
	if len(result.Breaks) != 1 {
		t.Fatalf("got %d breaks, want 1", len(result.Breaks))
	}
	if result.Breaks[0].EntryID != "id-1" {
		t.Errorf("got break at entry %q, want id-1", result.Breaks[0].EntryID)
	}
}

// TestVerifyChain_Epoch1_Intact verifies that epoch-1 entries with full payload
// pass chain verification when their hashes match the golden values.
func TestVerifyChain_Epoch1_Intact(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)
	cv := int32(3)

	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{
				{
					ID: "id-1", TenantID: "tenant-abc", Actor: "admin@example.com",
					Action: "set_field", ObjectKind: "field", EntryHash: goldenE1H1,
					CreatedAt: t1, ChainEpoch: 1,
					FieldPath: "payments.fee", OldValue: "0.02", NewValue: "0.03",
					ConfigVersion: &cv,
				},
				{
					ID: "id-0", TenantID: "tenant-abc", Actor: "admin@example.com",
					Action: "set_field", ObjectKind: "field", EntryHash: goldenE1H0,
					CreatedAt: t0, ChainEpoch: 1,
					FieldPath: "payments.fee", OldValue: "0.01", NewValue: "0.02",
					ConfigVersion: &cv, Metadata: []byte(`{"source":"api"}`),
				},
			},
		}, nil
	}

	result, err := client.VerifyChain(context.Background(), "tenant-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK chain, got breaks: %+v", result.Breaks)
	}
	if result.Total != 2 {
		t.Errorf("got Total %d, want 2", result.Total)
	}
}

// TestVerifyChain_Epoch1_PayloadTampered verifies that changing a payload field
// (e.g. new_value) is detected with epoch-1 hashing, even if structural fields
// are intact.
func TestVerifyChain_Epoch1_PayloadTampered(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	cv := int32(3)

	// Same entry as id-0 in the epoch-1 golden chain, but new_value is tampered.
	ma.queryWriteLogFn = func(_ context.Context, _ *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{
				{
					ID: "id-0", TenantID: "tenant-abc", Actor: "admin@example.com",
					Action: "set_field", ObjectKind: "field",
					// Hash is the correct goldenE1H0 but the payload has been tampered
					// (new_value changed from "0.02" to "9.99"). Verification must fail.
					EntryHash: goldenE1H0,
					CreatedAt: t0, ChainEpoch: 1,
					FieldPath:     "payments.fee",
					OldValue:      "0.01",
					NewValue:      "9.99", // tampered
					ConfigVersion: &cv,
					Metadata:      []byte(`{"source":"api"}`),
				},
			},
		}, nil
	}

	result, err := client.VerifyChain(context.Background(), "tenant-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OK {
		t.Error("expected chain not OK: payload was tampered but epoch-1 hash should catch it")
	}
	if len(result.Breaks) != 1 {
		t.Fatalf("got %d breaks, want 1", len(result.Breaks))
	}
	if result.Breaks[0].EntryID != "id-0" {
		t.Errorf("got break at entry %q, want id-0", result.Breaks[0].EntryID)
	}
}
