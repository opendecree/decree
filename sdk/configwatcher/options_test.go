package configwatcher

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestNew_Defaults(t *testing.T) {
	w := New(nil, "tenant-1")
	assert.Equal(t, "tenant-1", w.tenantID)
	assert.Equal(t, "superadmin", w.opts.role)
	assert.Equal(t, 500*time.Millisecond, w.opts.minBackoff)
	assert.Equal(t, 30*time.Second, w.opts.maxBackoff)
	assert.NotNil(t, w.opts.logger)
	assert.NotNil(t, w.fields)
}

func TestWithSubject(t *testing.T) {
	w := New(nil, "t1", WithSubject("alice"))
	assert.Equal(t, "alice", w.opts.subject)
}

func TestWithRole(t *testing.T) {
	w := New(nil, "t1", WithRole("admin"))
	assert.Equal(t, "admin", w.opts.role)
}

func TestWithTenantID(t *testing.T) {
	w := New(nil, "t1", WithTenantID("override"))
	assert.Equal(t, "override", w.opts.tenantID)
}

func TestWithBearerToken(t *testing.T) {
	w := New(nil, "t1", WithBearerToken("jwt"))
	assert.Equal(t, "jwt", w.opts.bearerToken)
}

func TestWithReconnectBackoff(t *testing.T) {
	w := New(nil, "t1", WithReconnectBackoff(1*time.Second, 1*time.Minute))
	assert.Equal(t, 1*time.Second, w.opts.minBackoff)
	assert.Equal(t, 1*time.Minute, w.opts.maxBackoff)
}

func TestWithLogger(t *testing.T) {
	l := slog.Default()
	w := New(nil, "t1", WithLogger(l))
	assert.Equal(t, l, w.opts.logger)
}

func TestWithAuth_MetadataHeaders(t *testing.T) {
	w := New(nil, "t1", WithSubject("alice"), WithRole("admin"), WithTenantID("t2"))
	ctx := w.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"alice"}, md.Get("x-subject"))
	assert.Equal(t, []string{"admin"}, md.Get("x-role"))
	assert.Equal(t, []string{"t2"}, md.Get("x-tenant-id"))
}

func TestWithAuth_BearerToken(t *testing.T) {
	w := New(nil, "t1", WithBearerToken("jwt"), WithSubject("alice"))
	ctx := w.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"Bearer jwt"}, md.Get("authorization"))
	assert.Empty(t, md.Get("x-subject"))
}

func TestWithAuth_NoOptions(t *testing.T) {
	w := &Watcher{opts: options{}}
	ctx := w.withAuth(context.Background())
	_, ok := metadata.FromOutgoingContext(ctx)
	assert.False(t, ok)
}

func TestFieldRegistration(t *testing.T) {
	w := New(nil, "t1")

	strVal := w.String("app.name", "default")
	intVal := w.Int("app.retries", 3)
	floatVal := w.Float("app.rate", 0.01)
	boolVal := w.Bool("app.enabled", false)
	durVal := w.Duration("app.timeout", time.Second)
	rawVal := w.Raw("app.raw", "raw-default")

	assert.Equal(t, "default", strVal.Get())
	assert.Equal(t, int64(3), intVal.Get())
	assert.Equal(t, 0.01, floatVal.Get())
	assert.False(t, boolVal.Get())
	assert.Equal(t, time.Second, durVal.Get())
	assert.Equal(t, "raw-default", rawVal.Get())

	paths := w.registeredPaths()
	assert.Len(t, paths, 6)
}

func TestValue_Close(t *testing.T) {
	v := newValue("default", parseString)
	v.close()

	// Channel should be closed — range will exit.
	count := 0
	for range v.Changes() {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestValue_ChannelOverflow(t *testing.T) {
	v := newValue(int64(0), parseInt)

	// Fill the channel (capacity 16).
	for i := range 20 {
		v.update(fmt.Sprintf("%d", i), true)
	}

	// Should still have a value — not stuck.
	assert.Equal(t, int64(19), v.Get())
}
