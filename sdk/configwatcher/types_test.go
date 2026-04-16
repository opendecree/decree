package configwatcher

import (
	"testing"
	"time"
)

func TestParseFunctions(t *testing.T) {
	t.Run("parseString", func(t *testing.T) {
		v, err := parseString("hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != "hello" {
			t.Errorf("got %v, want %v", got, "hello")
		}
	})

	t.Run("parseInt valid", func(t *testing.T) {
		v, err := parseInt("42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != int64(42) {
			t.Errorf("got %v, want %v", got, int64(42))
		}
	})

	t.Run("parseInt invalid", func(t *testing.T) {
		_, err := parseInt("abc")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("parseFloat valid", func(t *testing.T) {
		v, err := parseFloat("3.14")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != 3.14 {
			t.Errorf("got %v, want %v", got, 3.14)
		}
	})

	t.Run("parseFloat invalid", func(t *testing.T) {
		_, err := parseFloat("abc")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("parseBool valid", func(t *testing.T) {
		v, err := parseBool("true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !v {
			t.Error("expected true, got false")
		}
	})

	t.Run("parseBool invalid", func(t *testing.T) {
		_, err := parseBool("maybe")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("parseDuration valid", func(t *testing.T) {
		v, err := parseDuration("5m30s")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != 5*time.Minute+30*time.Second {
			t.Errorf("got %v, want %v", got, 5*time.Minute+30*time.Second)
		}
	})

	t.Run("parseDuration invalid", func(t *testing.T) {
		_, err := parseDuration("nope")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
