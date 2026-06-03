// Package testutil provides small helpers for writing test fixtures.
// It is only imported by *_test.go files and carries no production dependencies.
package testutil

import "testing"

// Must unwraps a (value, error) pair. If err is non-nil, it calls t.Fatal
// pointing at the caller's frame via t.Helper(), then returns the zero value
// of T. Use it to tighten fixture setup:
//
//	v, err := someFunc()
//	return testutil.Must(t, v, err)
func Must[T any](t testing.TB, v T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return v
}
