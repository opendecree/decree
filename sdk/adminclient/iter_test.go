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

// serverFixtureChain holds a 3-entry chain whose hashes were produced by the
// server's ComputeEntryHash (epoch 1, with payload and metadata).  The values
// were pre-computed with the same algorithm used by internal/audit/chain.go and
// are intentionally hardcoded here so the test cannot pass tautologically (i.e.
// VerifyChain cannot "cheat" by re-using its own hash function to generate the
// expected values).
//
// Computation inputs:
//
//	t0 = 2024-01-01T00:00:00Z (UnixNano = 1704067200000000000)
//	t1 = t0 + 1s
//	t2 = t0 + 2s
//
// Entry 0 (oldest): id=srv-id-0, tenant=t1, actor=actor@example.com,
//
//	action=set_field, object_kind=field, epoch=1,
//	field_path="app.name", old="old-value", new="new-value",
//	config_version=3, metadata={"env":"prod"}
//
// Entry 1: id=srv-id-1, … new="newer-value", config_version=4,
//
//	metadata={"env":"prod","region":"us-east-1"}
//
// Entry 2 (newest): id=srv-id-2, all payload fields nil/absent.
var serverFixtureChain = struct {
	t0, t1, t2 time.Time
	h0, h1, h2 string
	cv3, cv4   int32
}{
	t0: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	t1: time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
	t2: time.Date(2024, 1, 1, 0, 0, 2, 0, time.UTC),
	// Hashes produced by internal/audit.ComputeEntryHash (epoch 1).
	h0:  "224e3a5192d91f89e4e17cf72142c4a9ef549dcd897a7e2884f0c6a360603080",
	h1:  "d410a6a5aad0938af621e270839c2fd1a85cb15f6470a9f4190f09daa9db573c",
	h2:  "641579c51979ff8af817b204c6bb04477e5844f6c1fd3cf0b2f8e9837b0e7751",
	cv3: 3,
	cv4: 4,
}

// TestVerifyChain_MultiPageIntact tests the chunked streaming path where the
// server splits entries across multiple pages (newest first) using epoch-1
// hashes pre-computed by the server's ComputeEntryHash.
func TestVerifyChain_MultiPageIntact(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	fix := serverFixtureChain

	// Server returns two pages, newest first.
	// Page 1: [srv-id-2], Page 2: [srv-id-1, srv-id-0]
	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries: []*AuditEntry{{
					ID: "srv-id-2", TenantID: "t1", Actor: "actor@example.com",
					Action: "set_field", ObjectKind: "field",
					EntryHash: fix.h2, CreatedAt: fix.t2,
					ChainEpoch: 1,
				}},
				NextPageToken: "p2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{
				{
					ID: "srv-id-1", TenantID: "t1", Actor: "actor@example.com",
					Action: "set_field", ObjectKind: "field",
					EntryHash: fix.h1, CreatedAt: fix.t1,
					ChainEpoch: 1, FieldPath: "app.name",
					OldValue: "new-value", NewValue: "newer-value",
					ConfigVersion: &fix.cv4,
					Metadata:      map[string]string{"env": "prod", "region": "us-east-1"},
				},
				{
					ID: "srv-id-0", TenantID: "t1", Actor: "actor@example.com",
					Action: "set_field", ObjectKind: "field",
					EntryHash: fix.h0, CreatedAt: fix.t0,
					ChainEpoch: 1, FieldPath: "app.name",
					OldValue: "old-value", NewValue: "new-value",
					ConfigVersion: &fix.cv3,
					Metadata:      map[string]string{"env": "prod"},
				},
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
// span multiple pages, using server-format epoch-1 hashes as the fixture.
func TestVerifyChain_MultiPageTampered(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))

	fix := serverFixtureChain
	// srv-id-1 has a tampered hash that is not linked to h0 at all.
	badHash := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries: []*AuditEntry{{
					ID: "srv-id-1", TenantID: "t1", Actor: "actor@example.com",
					Action: "set_field", ObjectKind: "field",
					EntryHash: badHash, CreatedAt: fix.t1,
					ChainEpoch: 1, FieldPath: "app.name",
					OldValue: "new-value", NewValue: "newer-value",
					ConfigVersion: &fix.cv4,
					Metadata:      map[string]string{"env": "prod", "region": "us-east-1"},
				}},
				NextPageToken: "p2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{{
				ID: "srv-id-0", TenantID: "t1", Actor: "actor@example.com",
				Action: "set_field", ObjectKind: "field",
				EntryHash: fix.h0, CreatedAt: fix.t0,
				ChainEpoch: 1, FieldPath: "app.name",
				OldValue: "old-value", NewValue: "new-value",
				ConfigVersion: &fix.cv3,
				Metadata:      map[string]string{"env": "prod"},
			}},
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
	if result.Breaks[0].EntryID != "srv-id-1" {
		t.Errorf("got break at entry %q, want srv-id-1", result.Breaks[0].EntryID)
	}
}
