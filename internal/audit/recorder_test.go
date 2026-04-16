package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRecorderLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func newTestRecorder(store *MemoryStore) *UsageRecorder {
	return NewUsageRecorder(store, RecorderConfig{
		FlushInterval: time.Hour, // manual flush in tests
		Logger:        testRecorderLogger,
	})
}

func TestRecordRead_SingleField(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)
	actor := "alice"

	r.RecordRead("t1", "app.fee", &actor)
	require.NoError(t, r.Flush(context.Background()))

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(1), stats[0].ReadCount)
	assert.Equal(t, &actor, stats[0].LastReadBy)
}

func TestRecordReads_MultipleFields(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	r.RecordReads("t1", []string{"a.x", "a.y", "a.z"}, nil)
	require.NoError(t, r.Flush(context.Background()))

	for _, path := range []string{"a.x", "a.y", "a.z"} {
		stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
			TenantID:  "t1",
			FieldPath: path,
		})
		require.NoError(t, err)
		require.Len(t, stats, 1, "path %s", path)
		assert.Equal(t, int64(1), stats[0].ReadCount)
	}
}

func TestRecordRead_Accumulates(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	for i := 0; i < 100; i++ {
		r.RecordRead("t1", "app.fee", nil)
	}
	require.NoError(t, r.Flush(context.Background()))

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(100), stats[0].ReadCount)
}

func TestRecordRead_LastReaderTracked(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)
	alice := "alice"
	bob := "bob"

	r.RecordRead("t1", "app.fee", &alice)
	r.RecordRead("t1", "app.fee", &bob)
	require.NoError(t, r.Flush(context.Background()))

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(2), stats[0].ReadCount)
	assert.Equal(t, &bob, stats[0].LastReadBy)
}

func TestRecordRead_MultiTenant(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	r.RecordRead("t1", "app.fee", nil)
	r.RecordRead("t2", "app.fee", nil)
	require.NoError(t, r.Flush(context.Background()))

	for _, tid := range []string{"t1", "t2"} {
		stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
			TenantID:  tid,
			FieldPath: "app.fee",
		})
		require.NoError(t, err)
		require.Len(t, stats, 1, "tenant %s", tid)
		assert.Equal(t, int64(1), stats[0].ReadCount)
	}
}

func TestFlush_SwapsBuffer(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	r.RecordRead("t1", "app.fee", nil)
	require.NoError(t, r.Flush(context.Background()))

	// Second batch.
	r.RecordRead("t1", "app.fee", nil)
	require.NoError(t, r.Flush(context.Background()))

	// MemoryStore accumulates via upsert: total should be 2.
	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(2), stats[0].ReadCount)
}

func TestFlush_EmptyBuffer(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	// Flush with nothing pending — should be a no-op.
	require.NoError(t, r.Flush(context.Background()))

	stats, err := store.GetTenantUsage(context.Background(), GetTenantUsageParams{TenantID: "t1"})
	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestFlush_StoreError(t *testing.T) {
	store := &failingStore{MemoryStore: NewMemoryStore(), err: errors.New("db down")}
	r := NewUsageRecorder(store, RecorderConfig{
		FlushInterval: time.Hour,
		Logger:        testRecorderLogger,
	})

	r.RecordRead("t1", "app.fee", nil)
	err := r.Flush(context.Background())
	assert.Error(t, err)
}

func TestNilRecorder_SafeToCall(t *testing.T) {
	var r *UsageRecorder

	// None of these should panic.
	r.RecordRead("t1", "app.fee", nil)
	r.RecordReads("t1", []string{"a", "b"}, nil)
	assert.NoError(t, r.Flush(context.Background()))
	r.Stop()
}

func TestAutoFlush(t *testing.T) {
	store := NewMemoryStore()
	r := NewUsageRecorder(store, RecorderConfig{
		FlushInterval: 20 * time.Millisecond,
		Logger:        testRecorderLogger,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go r.Start(ctx)

	r.RecordRead("t1", "app.fee", nil)

	// Wait for at least one auto-flush.
	time.Sleep(80 * time.Millisecond)

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(1), stats[0].ReadCount)

	cancel()
	r.Stop()
}

func TestStop_FinalFlush(t *testing.T) {
	store := NewMemoryStore()
	r := NewUsageRecorder(store, RecorderConfig{
		FlushInterval: time.Hour, // won't auto-flush
		Logger:        testRecorderLogger,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go r.Start(ctx)

	r.RecordRead("t1", "app.fee", nil)

	// Cancel and stop — should trigger final flush.
	cancel()
	r.Stop()

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(1), stats[0].ReadCount)
}

func TestPeriodBucketing(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	r.RecordRead("t1", "app.fee", nil)
	require.NoError(t, r.Flush(context.Background()))

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)

	// Period start should be truncated to the current hour.
	expected := time.Now().UTC().Truncate(time.Hour)
	assert.Equal(t, expected, stats[0].PeriodStart)
}

func TestRecordRead_Concurrent(t *testing.T) {
	store := NewMemoryStore()
	r := newTestRecorder(store)

	const goroutines = 10
	const readsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			actor := fmt.Sprintf("worker-%d", id)
			for range readsPerGoroutine {
				r.RecordRead("t1", "app.fee", &actor)
			}
		}(i)
	}
	wg.Wait()

	require.NoError(t, r.Flush(context.Background()))

	stats, err := store.GetFieldUsage(context.Background(), GetFieldUsageParams{
		TenantID:  "t1",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(goroutines*readsPerGoroutine), stats[0].ReadCount)
}

// failingStore is a Store implementation that always returns an error on UpsertUsageStats.
type failingStore struct {
	*MemoryStore
	err error
}

func (s *failingStore) UpsertUsageStats(_ context.Context, _ UpsertUsageStatsParams) error {
	return s.err
}
