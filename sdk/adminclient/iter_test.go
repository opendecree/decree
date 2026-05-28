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

// TestVerifyChain_MultiPageIntact tests the chunked streaming path where the
// server splits entries across multiple pages (newest first).
func TestVerifyChain_MultiPageIntact(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)
	t2 := t0.Add(2 * time.Second)

	h0 := computeClientHash("", "id-0", "t1", "actor", "set_field", "field", t0)
	h1 := computeClientHash(h0, "id-1", "t1", "actor", "set_field", "field", t1)
	h2 := computeClientHash(h1, "id-2", "t1", "actor", "set_field", "field", t2)

	// Server returns two pages, newest first.
	// Page 1: [id-2], Page 2: [id-1, id-0]
	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "id-2", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: h2, CreatedAt: t2}},
				NextPageToken: "p2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{
				{ID: "id-1", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: h1, CreatedAt: t1},
				{ID: "id-0", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: h0, CreatedAt: t0},
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

// TestVerifyChain_MultiPageTampered verifies tampering is detected when entries
// span multiple pages.
func TestVerifyChain_MultiPageTampered(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	h0 := computeClientHash("", "id-0", "t1", "actor", "set_field", "field", t0)
	// id-1 has a tampered hash (not linked to h0).
	badHash := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "id-1", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: badHash, CreatedAt: t1}},
				NextPageToken: "p2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{{ID: "id-0", TenantID: "t1", Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: h0, CreatedAt: t0}},
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
