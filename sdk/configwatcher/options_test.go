package configwatcher

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

func TestNew_Defaults(t *testing.T) {
	w := New(nil, "tenant-1")
	if got := w.tenantID; got != "tenant-1" {
		t.Errorf("got %v, want %v", got, "tenant-1")
	}
	if got := w.opts.role; got != "superadmin" {
		t.Errorf("got %v, want %v", got, "superadmin")
	}
	if got := w.opts.minBackoff; got != 500*time.Millisecond {
		t.Errorf("got %v, want %v", got, 500*time.Millisecond)
	}
	if got := w.opts.maxBackoff; got != 30*time.Second {
		t.Errorf("got %v, want %v", got, 30*time.Second)
	}
	if w.opts.logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if w.fields == nil {
		t.Fatal("expected non-nil fields")
	}
}

func TestWithSubject(t *testing.T) {
	w := New(nil, "t1", WithSubject("alice"))
	if got := w.opts.subject; got != "alice" {
		t.Errorf("got %v, want %v", got, "alice")
	}
}

func TestWithRole(t *testing.T) {
	w := New(nil, "t1", WithRole("admin"))
	if got := w.opts.role; got != "admin" {
		t.Errorf("got %v, want %v", got, "admin")
	}
}

func TestWithTenantID(t *testing.T) {
	w := New(nil, "t1", WithTenantID("override"))
	if got := w.opts.tenantID; got != "override" {
		t.Errorf("got %v, want %v", got, "override")
	}
}

func TestWithBearerToken(t *testing.T) {
	w := New(nil, "t1", WithBearerToken("jwt"))
	if got := w.opts.bearerToken; got != "jwt" {
		t.Errorf("got %v, want %v", got, "jwt")
	}
}

func TestWithReconnectBackoff(t *testing.T) {
	w := New(nil, "t1", WithReconnectBackoff(1*time.Second, 1*time.Minute))
	if got := w.opts.minBackoff; got != 1*time.Second {
		t.Errorf("got %v, want %v", got, 1*time.Second)
	}
	if got := w.opts.maxBackoff; got != 1*time.Minute {
		t.Errorf("got %v, want %v", got, 1*time.Minute)
	}
}

func TestWithLogger(t *testing.T) {
	l := slog.Default()
	w := New(nil, "t1", WithLogger(l))
	if got := w.opts.logger; got != l {
		t.Errorf("got %v, want %v", got, l)
	}
}

func TestWithAuth_MetadataHeaders(t *testing.T) {
	w := New(nil, "t1", WithSubject("alice"), WithRole("admin"), WithTenantID("t2"))
	ctx := w.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if got := md.Get("x-subject"); !reflect.DeepEqual(got, []string{"alice"}) {
		t.Errorf("got %v, want %v", got, []string{"alice"})
	}
	if got := md.Get("x-role"); !reflect.DeepEqual(got, []string{"admin"}) {
		t.Errorf("got %v, want %v", got, []string{"admin"})
	}
	if got := md.Get("x-tenant-id"); !reflect.DeepEqual(got, []string{"t2"}) {
		t.Errorf("got %v, want %v", got, []string{"t2"})
	}
}

func TestWithAuth_BearerToken(t *testing.T) {
	w := New(nil, "t1", WithBearerToken("jwt"), WithSubject("alice"))
	ctx := w.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if got := md.Get("authorization"); !reflect.DeepEqual(got, []string{"Bearer jwt"}) {
		t.Errorf("got %v, want %v", got, []string{"Bearer jwt"})
	}
	if got := md.Get("x-subject"); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestWithAuth_NoOptions(t *testing.T) {
	w := &Watcher{opts: options{}}
	ctx := w.withAuth(context.Background())
	_, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		t.Error("expected no outgoing metadata")
	}
}

func TestFieldRegistration(t *testing.T) {
	w := New(nil, "t1")

	strVal := w.String("app.name", "default")
	intVal := w.Int("app.retries", 3)
	floatVal := w.Float("app.rate", 0.01)
	boolVal := w.Bool("app.enabled", false)
	durVal := w.Duration("app.timeout", time.Second)
	rawVal := w.Raw("app.raw", "raw-default")

	if got := strVal.Get(); got != "default" {
		t.Errorf("got %v, want %v", got, "default")
	}
	if got := intVal.Get(); got != int64(3) {
		t.Errorf("got %v, want %v", got, int64(3))
	}
	if got := floatVal.Get(); got != 0.01 {
		t.Errorf("got %v, want %v", got, 0.01)
	}
	if boolVal.Get() {
		t.Error("expected false, got true")
	}
	if got := durVal.Get(); got != time.Second {
		t.Errorf("got %v, want %v", got, time.Second)
	}
	if got := rawVal.Get(); got != "raw-default" {
		t.Errorf("got %v, want %v", got, "raw-default")
	}

	paths := w.registeredPaths()
	if len(paths) != 6 {
		t.Errorf("got len %d, want %d", len(paths), 6)
	}
}

func TestValue_Close(t *testing.T) {
	v := newValue("default", parseString)
	v.close()

	// Channel should be closed — range will exit.
	count := 0
	for range v.Changes() {
		count++
	}
	if got := count; got != 0 {
		t.Errorf("got %v, want %v", got, 0)
	}
}

func TestValue_ChannelOverflow(t *testing.T) {
	v := newValue(int64(0), parseInt)

	// Fill the channel (capacity 16).
	for i := range 20 {
		v.update(fmt.Sprintf("%d", i), true)
	}

	// Should still have a value — not stuck.
	if got := v.Get(); got != int64(19) {
		t.Errorf("got %v, want %v", got, int64(19))
	}
}
