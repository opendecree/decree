package configwatcher

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	w := New(nil, "tenant-1")
	if got := w.tenantID; got != "tenant-1" {
		t.Errorf("got %v, want %v", got, "tenant-1")
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
