package pagination

import (
	"encoding/base64"
	"testing"
)

func TestDecodePageToken_Empty(t *testing.T) {
	offset, err := DecodePageToken("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offset != 0 {
		t.Errorf("got %d, want 0", offset)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for _, offset := range []int32{0, 1, 50, 100, 999} {
		token := EncodePageToken(offset)
		got, err := DecodePageToken(token)
		if err != nil {
			t.Fatalf("DecodePageToken(%q): %v", token, err)
		}
		if got != offset {
			t.Errorf("round-trip offset %d: got %d", offset, got)
		}
	}
}

// TestDecodePageToken_Tampered verifies that modifying the base64 payload of a
// valid token causes decoding to fail. Tokens are format-validated (version
// prefix + numeric offset) but not HMAC-signed, so only structural corruption
// is guaranteed to be detected.
func TestDecodePageToken_Tampered(t *testing.T) {
	token := EncodePageToken(42)

	// Corrupt the version prefix inside the payload: decode → mangle → re-encode.
	raw, _ := base64.StdEncoding.DecodeString(token)
	raw[0] ^= 0xFF // flip all bits of the first byte — breaks the "v1:" prefix
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := DecodePageToken(tampered); err == nil {
		t.Errorf("expected error for tampered token %q, got nil", tampered)
	}

	// Corrupt the version string by bumping the version digit.
	corrupted := base64.StdEncoding.EncodeToString([]byte("v2:42"))
	if _, err := DecodePageToken(corrupted); err == nil {
		t.Errorf("expected error for wrong-version token %q, got nil", corrupted)
	}

	// Truncate the base64 to an incomplete token.
	if len(token) > 2 {
		if _, err := DecodePageToken(token[:len(token)-2]); err == nil {
			t.Errorf("expected error for truncated token, got nil")
		}
	}
}

func TestDecodePageToken_Invalid(t *testing.T) {
	tests := []string{
		"not-base64!!!",
		"aW52YWxpZA==", // "invalid" (no version prefix)
		"djI6NTA=",     // "v2:50" (wrong version)
		"djE6LTE=",     // "v1:-1" (negative offset)
		"djE6YWJj",     // "v1:abc" (non-numeric)
	}
	for _, token := range tests {
		_, err := DecodePageToken(token)
		if err == nil {
			t.Errorf("DecodePageToken(%q): expected error, got nil", token)
		}
	}
}

func TestClampPageSize(t *testing.T) {
	// Zero/negative → default
	if got := ClampPageSize(0, 50, 100); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
	if got := ClampPageSize(-1, 50, 100); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
	// Within range → as-is
	if got := ClampPageSize(25, 50, 100); got != 25 {
		t.Errorf("got %d, want 25", got)
	}
	// Over max → capped
	if got := ClampPageSize(200, 50, 100); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
	// Different max for lightweight resources
	if got := ClampPageSize(300, 50, 500); got != 300 {
		t.Errorf("got %d, want 300", got)
	}
	if got := ClampPageSize(600, 50, 500); got != 500 {
		t.Errorf("got %d, want 500", got)
	}
}

func TestNextPageToken(t *testing.T) {
	// Full page → has next
	if token := NextPageToken(50, 51, 0); token == "" {
		t.Error("expected non-empty token for full page")
	}

	// Partial page → no next
	if token := NextPageToken(50, 30, 0); token != "" {
		t.Errorf("expected empty token for partial page, got %q", token)
	}

	// Exact page → no next
	if token := NextPageToken(50, 50, 0); token != "" {
		t.Errorf("expected empty token for exact page, got %q", token)
	}

	// Second page token encodes correct offset
	token := NextPageToken(50, 51, 0)
	offset, err := DecodePageToken(token)
	if err != nil {
		t.Fatalf("DecodePageToken: %v", err)
	}
	if offset != 50 {
		t.Errorf("got offset %d, want 50", offset)
	}

	// Third page from offset 50
	token = NextPageToken(50, 51, 50)
	offset, err = DecodePageToken(token)
	if err != nil {
		t.Fatalf("DecodePageToken: %v", err)
	}
	if offset != 100 {
		t.Errorf("got offset %d, want 100", offset)
	}
}
