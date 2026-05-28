package config

// Concurrency proofs for the errgroup parallel reads in service.go.
//
// Each test uses a channel-gate pattern:
//  1. Replace a store method with one that signals "started" then blocks on "proceed".
//  2. Launch the function under test in a goroutine.
//  3. Wait for both "started" channels (proves both goroutines were launched).
//  4. Verify no "proceed" has fired yet (proves they are both blocked, i.e. concurrent).
//  5. Unblock both, let the function finish, assert the result.
//
// If either goroutine runs sequentially the test deadlocks: the second
// "started" never fires because the first goroutine is still holding the
// only available "proceed" slot.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/storage/domain"
)

const testTimeout = 3 * time.Second

// gateStore wraps mockStore and overrides GetConfigValueAtVersion so that
// each call signals a per-call "started" channel then blocks until the
// matching "proceed" channel is closed.
type gateStore struct {
	mockStore
	mu sync.Mutex
	// Indexed by call order: gates[0] is first call, gates[1] second, etc.
	gates []callGate
	// callIdx tracks how many calls have been dispatched so far.
	callIdx int
}

type callGate struct {
	started chan struct{}
	proceed chan struct{}
}

func newGate() callGate {
	return callGate{
		started: make(chan struct{}),
		proceed: make(chan struct{}),
	}
}

func (s *gateStore) GetConfigValueAtVersion(ctx context.Context, arg GetConfigValueAtVersionParams) (GetConfigValueAtVersionRow, error) {
	// Grab the next gate for this call under the lock, then release before blocking.
	s.mu.Lock()
	idx := s.callIdx
	s.callIdx++
	s.mu.Unlock()

	if idx >= len(s.gates) {
		// Fall back to the embedded mock for any extra calls (e.g. filterByImportMode).
		return s.mockStore.GetConfigValueAtVersion(ctx, arg)
	}
	g := s.gates[idx]
	close(g.started) // signal: this goroutine has started
	select {
	case <-g.proceed: // unblocked by the test
	case <-ctx.Done():
	}
	return s.mockStore.GetConfigValueAtVersion(ctx, arg)
}

// TestGetFieldsFanOut_RunsConcurrently proves that the per-field errgroup in
// GetFields launches all field reads at the same time. If the reads were
// serialized the second goroutine would never reach "started" while the first
// is blocked on "proceed", causing a deadlock (caught by the test timeout).
func TestGetFieldsFanOut_RunsConcurrently(t *testing.T) {
	t.Parallel()

	g0, g1 := newGate(), newGate()
	gs := &gateStore{gates: []callGate{g0, g1}}

	// Wire up the mockStore embedded in gateStore for everything except
	// GetConfigValueAtVersion (which gateStore overrides).
	ctx := auth.WithoutAuth(context.Background())

	gs.mockStore.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// GetConfigValueAtVersion is called by gateStore — set up the embedded
	// mock for the underlying response after the gate unblocks.
	val := "v1"
	chk := "c1"
	gs.mockStore.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{FieldPath: "a.x", Value: &val, Checksum: &chk}, nil)

	// Also need the cache and publisher.
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	// Pass gs as the Store: gateStore overrides GetConfigValueAtVersion,
	// all other methods fall through to gs.mockStore.
	svc := NewService(gs, c, pub, sub, WithLogger(testLogger))

	// Cache returns miss so the service proceeds to the fan-out.
	c.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)

	type result struct {
		resp *pb.GetFieldsResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
			TenantId:   tenantID1,
			FieldPaths: []string{"a.x", "a.y"},
		})
		done <- result{resp, err}
	}()

	deadline := time.NewTimer(testTimeout)
	defer deadline.Stop()

	// Step 1: wait for both goroutines to start.
	for _, started := range []chan struct{}{g0.started, g1.started} {
		select {
		case <-started:
		case <-deadline.C:
			t.Fatal("timeout: not both field-read goroutines started — reads may be sequential")
		}
	}

	// Step 2: verify no proceed has fired (both are blocked concurrently).
	select {
	case <-done:
		t.Fatal("GetFields completed before proceeds were released — reads were not concurrent")
	default:
	}

	// Step 3: unblock both goroutines.
	close(g0.proceed)
	close(g1.proceed)

	// Step 4: collect the result.
	select {
	case r := <-done:
		require.NoError(t, r.err)
		assert.Len(t, r.resp.Values, 2)
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for GetFields to complete after unblocking")
	}
}

// TestImportConfigChangeEventFanOut_RunsConcurrently proves that the
// change-event errgroup in ImportConfig launches old-value reads concurrently.
// If they were serialized the second goroutine would never reach "started"
// while the first is blocked, causing a deadlock caught by the test timeout.
func TestImportConfigChangeEventFanOut_RunsConcurrently(t *testing.T) {
	t.Parallel()

	g0, g1 := newGate(), newGate()
	gs := &gateStore{gates: []callGate{g0, g1}}

	ctx := superadminCtx()

	// Set up the embedded mockStore for all calls other than the gated ones.
	gs.mockStore.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	gs.mockStore.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	gs.mockStore.On("GetSchemaFields", mock.Anything, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "f.a", FieldType: domain.FieldTypeString},
			{Path: "f.b", FieldType: domain.FieldTypeString},
		}, nil)
	gs.mockStore.On("GetFieldLocks", mock.Anything, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	gs.mockStore.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)

	// The gated GetConfigValueAtVersion calls — return not-found after unblocking.
	gs.mockStore.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	newVer := domain.ConfigVersion{ID: versionID20, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}
	gs.mockStore.On("RunInTx", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(Store) error)
		_ = fn(&gs.mockStore)
	})
	gs.mockStore.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(newVer, nil)
	gs.mockStore.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	gs.mockStore.On("BulkInsertAuditWriteLog", mock.Anything, mock.Anything).Return(nil)
	gs.mockStore.On("GetFullConfigAtVersion", mock.Anything, mock.Anything).
		Return([]GetFullConfigAtVersionRow{}, nil)

	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	c.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	svc := NewService(gs, c, pub, sub, WithLogger(testLogger))

	yamlContent := []byte(`spec_version: "v1"
values:
  f.a:
    value: "hello"
  f.b:
    value: "world"
`)

	type result struct {
		resp *pb.ImportConfigResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
			TenantId:    tenantID1,
			YamlContent: yamlContent,
			// REPLACE skips filterByImportMode's sequential getCurrentValue calls,
			// ensuring the first gated GetConfigValueAtVersion calls come from the
			// parallel change-event fan-out, not from the filter phase.
			Mode: pb.ImportMode_IMPORT_MODE_REPLACE,
		})
		done <- result{resp, err}
	}()

	deadline := time.NewTimer(testTimeout)
	defer deadline.Stop()

	// Step 1: wait for both old-value goroutines to start.
	for _, started := range []chan struct{}{g0.started, g1.started} {
		select {
		case <-started:
		case <-deadline.C:
			t.Fatal("timeout: not both change-event goroutines started — reads may be sequential")
		}
	}

	// Step 2: verify the function is still blocked (both goroutines running).
	select {
	case <-done:
		t.Fatal("ImportConfig completed before proceeds were released — reads were not concurrent")
	default:
	}

	// Step 3: unblock both goroutines.
	close(g0.proceed)
	close(g1.proceed)

	// Step 4: collect the result.
	select {
	case r := <-done:
		require.NoError(t, r.err)
		assert.Equal(t, int32(2), r.resp.ConfigVersion.Version)
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for ImportConfig to complete after unblocking")
	}
}

// TestChecksumGuardedWrites_ChecksumMismatchInTx verifies that a concurrent
// write that changes the latest version before our tx commits causes an Aborted
// error. It simulates the race with successive mock return values:
//   - Outside-tx GetLatestConfigVersion returns v1 (pre-race state).
//   - Inside-tx GetLatestConfigVersion returns v2 (post-concurrent-write state).
//   - The checksum at v2 differs from the client's expectedChecksum.
//
// This proves the checksum check is bound to the tx and detects the race (fix for #417).
func TestChecksumGuardedWrites_ChecksumMismatchInTx(t *testing.T) {
	t.Parallel()

	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub, WithLogger(testLogger))

	ctx := superadminCtx()
	expectedChecksum := "checksum-at-v1"
	differentChecksum := "checksum-at-v2"

	// First call: getOrCreateVersion outside the tx sees v1.
	// Second call: txLatestVersion inside the tx sees v2 (concurrent write committed).
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil).Once()
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 2}, nil).Once()

	// getCurrentValue (v1, outside tx) then checkChecksumAtVersion (v2, inside tx).
	// Both return a row but with different checksums across calls.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{Value: strPtr("old"), Checksum: &expectedChecksum}, nil).Once()
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{Value: strPtr("newer"), Checksum: &differentChecksum}, nil).Once()

	setupNoSensitiveFields(store)
	c.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
		FieldPath:        "app.env",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "prod"}},
		ExpectedChecksum: &expectedChecksum,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err), "checksum mismatch due to concurrent write must return Aborted")
}

// TestChecksumGuardedWrites_VersionConflictReturnsAborted verifies that when
// CreateConfigVersion returns ErrVersionConflict (two writers racing to create
// the same version slot), the service returns codes.Aborted rather than
// codes.Internal. This proves the UNIQUE constraint collision is surfaced as a
// retriable error, not an unexpected internal failure.
func TestChecksumGuardedWrites_VersionConflictReturnsAborted(t *testing.T) {
	t.Parallel()

	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub, WithLogger(testLogger))

	ctx := superadminCtx()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// getCurrentValue outside tx: field not found.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	setupNoSensitiveFields(store)
	// No ExpectedChecksum → checkChecksumAtVersion is skipped; CreateConfigVersion
	// fails immediately with ErrVersionConflict (simulating the UNIQUE collision).
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict)
	c.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.env",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "prod"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err), "version conflict must return Aborted, not Internal")
}
