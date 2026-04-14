package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryPubSub_PublishSubscribe(t *testing.T) {
	ps := NewMemoryPubSub()
	ctx := context.Background()

	ch, cancel, err := ps.Subscribe(ctx, "t1")
	require.NoError(t, err)
	defer cancel()

	event := ConfigChangeEvent{
		TenantID:  "t1",
		Version:   1,
		FieldPath: "app.fee",
		OldValue:  "0.01",
		NewValue:  "0.02",
		ChangedBy: "admin",
		ChangedAt: time.Now(),
	}
	require.NoError(t, ps.Publish(ctx, event))

	select {
	case got := <-ch:
		assert.Equal(t, "app.fee", got.FieldPath)
		assert.Equal(t, "0.02", got.NewValue)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event")
	}
}

func TestMemoryPubSub_TenantIsolation(t *testing.T) {
	ps := NewMemoryPubSub()
	ctx := context.Background()

	ch1, cancel1, _ := ps.Subscribe(ctx, "t1")
	defer cancel1()
	ch2, cancel2, _ := ps.Subscribe(ctx, "t2")
	defer cancel2()

	require.NoError(t, ps.Publish(ctx, ConfigChangeEvent{TenantID: "t1", FieldPath: "a"}))

	select {
	case <-ch1:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("t1 should receive event")
	}

	select {
	case <-ch2:
		t.Fatal("t2 should not receive t1 event")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestMemoryPubSub_MultipleSubscribers(t *testing.T) {
	ps := NewMemoryPubSub()
	ctx := context.Background()

	ch1, cancel1, _ := ps.Subscribe(ctx, "t1")
	defer cancel1()
	ch2, cancel2, _ := ps.Subscribe(ctx, "t1")
	defer cancel2()

	require.NoError(t, ps.Publish(ctx, ConfigChangeEvent{TenantID: "t1", FieldPath: "a"}))

	select {
	case <-ch1:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 1 should receive")
	}
	select {
	case <-ch2:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 2 should receive")
	}
}

func TestMemoryPubSub_Unsubscribe(t *testing.T) {
	ps := NewMemoryPubSub()
	ctx := context.Background()

	ch, cancel, _ := ps.Subscribe(ctx, "t1")
	cancel()

	// Channel should be closed.
	_, ok := <-ch
	assert.False(t, ok)
}

func TestMemoryPubSub_Close(t *testing.T) {
	ps := NewMemoryPubSub()
	ctx := context.Background()

	ch, _, _ := ps.Subscribe(ctx, "t1")
	require.NoError(t, ps.Close())

	_, ok := <-ch
	assert.False(t, ok)
}

func TestMemoryPubSub_PublishNoSubscribers(t *testing.T) {
	ps := NewMemoryPubSub()
	err := ps.Publish(context.Background(), ConfigChangeEvent{TenantID: "t1"})
	assert.NoError(t, err)
}
