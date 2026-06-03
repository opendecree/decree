package testutil_test

import (
	"errors"
	"testing"

	"github.com/opendecree/decree/internal/testutil"
)

// stubTB satisfies testing.TB for unit-testing Must without triggering
// runtime.Goexit (which the real Fatalf calls via FailNow).
type stubTB struct {
	testing.TB
	fatal  bool
	format string
}

func (s *stubTB) Helper()                               {}
func (s *stubTB) Fatalf(f string, _ ...any)             { s.fatal = true; s.format = f }
func (s *stubTB) Logf(_ string, _ ...any)               {}
func (s *stubTB) Log(_ ...any)                          {}
func (s *stubTB) Fail()                                 { s.fatal = true }
func (s *stubTB) FailNow()                              { s.fatal = true }
func (s *stubTB) Failed() bool                          { return s.fatal }
func (s *stubTB) Cleanup(_ func())                      {}
func (s *stubTB) TempDir() string                       { return "" }
func (s *stubTB) Setenv(_ string, _ string)             {}
func (s *stubTB) Chdir(_ string)                        {}
func (s *stubTB) Skipped() bool                         { return false }
func (s *stubTB) SkipNow()                              {}
func (s *stubTB) Skip(_ ...any)                         {}
func (s *stubTB) Skipf(_ string, _ ...any)              {}
func (s *stubTB) Error(_ ...any)                        { s.fatal = true }
func (s *stubTB) Errorf(_ string, _ ...any)             { s.fatal = true }
func (s *stubTB) Fatal(_ ...any)                        { s.fatal = true }
func (s *stubTB) Name() string                          { return "stub" }
func (s *stubTB) Parallel()                             {}
func (s *stubTB) Run(_ string, _ func(*testing.T)) bool { return true }

func TestMust_success(t *testing.T) {
	got := testutil.Must(t, 42, nil)
	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
}

func TestMust_failure(t *testing.T) {
	tb := &stubTB{}
	testutil.Must(tb, 0, errors.New("boom"))
	if !tb.fatal {
		t.Fatal("expected Must to call Fatalf on error")
	}
}

func TestMust_string(t *testing.T) {
	got := testutil.Must(t, "hello", nil)
	if got != "hello" {
		t.Fatalf("want 'hello', got %q", got)
	}
}
