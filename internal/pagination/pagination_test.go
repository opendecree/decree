package pagination

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
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

func TestDecodeTokenKind_Empty(t *testing.T) {
	kind, offset, cur, err := DecodeTokenKind("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != KindFirst {
		t.Errorf("got kind %v, want KindFirst", kind)
	}
	if offset != 0 {
		t.Errorf("got offset %d, want 0", offset)
	}
	_ = cur
}

func TestDecodeTokenKind_V1Offset(t *testing.T) {
	token := EncodePageToken(42)
	kind, offset, _, err := DecodeTokenKind(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != KindOffset {
		t.Errorf("got kind %v, want KindOffset", kind)
	}
	if offset != 42 {
		t.Errorf("got offset %d, want 42", offset)
	}
}

func TestDecodeTokenKind_V2Cursor(t *testing.T) {
	ts := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	id := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	token := EncodeCursorToken(PageCursor{Time: ts, ID: id})

	kind, _, cur, err := DecodeTokenKind(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != KindCursor {
		t.Errorf("got kind %v, want KindCursor", kind)
	}
	if !cur.Time.Equal(ts) {
		t.Errorf("got time %v, want %v", cur.Time, ts)
	}
	if cur.ID != id {
		t.Errorf("got id %q, want %q", cur.ID, id)
	}
}

func TestDecodeTokenKind_Invalid(t *testing.T) {
	tests := []string{
		"not-base64!!!",
		"aW52YWxpZA==", // "invalid"
		base64.StdEncoding.EncodeToString([]byte("v3:something")),   // unknown version
		base64.StdEncoding.EncodeToString([]byte("v2:abc:some-id")), // non-numeric ns
		base64.StdEncoding.EncodeToString([]byte("v2:123:")),        // empty id
	}
	for _, token := range tests {
		_, _, _, err := DecodeTokenKind(token)
		if err == nil {
			t.Errorf("DecodeTokenKind(%q): expected error, got nil", token)
		}
	}
}

func TestNextCursorToken_HasMore(t *testing.T) {
	ts := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	id := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	token := NextCursorToken(10, 11, ts, id)
	if token == "" {
		t.Fatal("expected non-empty cursor token")
	}
	kind, _, cur, err := DecodeTokenKind(token)
	if err != nil {
		t.Fatalf("DecodeTokenKind: %v", err)
	}
	if kind != KindCursor {
		t.Errorf("got kind %v, want KindCursor", kind)
	}
	if !cur.Time.Equal(ts) {
		t.Errorf("got time %v, want %v", cur.Time, ts)
	}
}

func TestNextCursorToken_NoMore(t *testing.T) {
	ts := time.Now()
	if token := NextCursorToken(10, 10, ts, "some-id"); token != "" {
		t.Errorf("expected empty token for exact page, got %q", token)
	}
	if token := NextCursorToken(10, 5, ts, "some-id"); token != "" {
		t.Errorf("expected empty token for partial page, got %q", token)
	}
}

// --- Iter ---

func TestIter_FullIteration(t *testing.T) {
	// Three pages of two items each (pageSize=2, store returns 2+1 to signal more).
	pages := [][]int{
		{1, 2, 3}, // fetch returns 3 to indicate "has next"; Iter trims to 2
		{3, 4, 3}, // same pattern
		{5, 6},    // last page: exactly 2 items, no next token
	}
	call := 0
	fetch := func(_ context.Context, token string) ([]int, string, error) {
		page := pages[call]
		call++
		const pageSize = int32(2)
		next := NextPageToken(pageSize, int32(len(page)), 0)
		if int32(len(page)) > pageSize {
			page = page[:pageSize]
		}
		return page, next, nil
	}

	var got []int
	for v, err := range Iter(context.Background(), fetch) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = append(got, v)
	}
	want := []int{1, 2, 3, 4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestIter_EarlyBreak(t *testing.T) {
	fetchCalls := 0
	fetch := func(_ context.Context, _ string) ([]int, string, error) {
		fetchCalls++
		return []int{fetchCalls * 10, fetchCalls*10 + 1}, EncodePageToken(int32(fetchCalls * 2)), nil
	}

	var got []int
	for v, err := range Iter(context.Background(), fetch) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = append(got, v)
		if len(got) == 3 {
			break
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 items after break, got %d", len(got))
	}
	// Only two pages should have been fetched (items 10,11 then 20 breaks mid-page).
	if fetchCalls > 2 {
		t.Errorf("expected ≤2 fetch calls after early break, got %d", fetchCalls)
	}
}

func TestIter_FetchError(t *testing.T) {
	sentinel := errors.New("store down")
	fetch := func(_ context.Context, _ string) ([]int, string, error) {
		return nil, "", sentinel
	}

	var gotErr error
	for _, err := range Iter(context.Background(), fetch) {
		gotErr = err
		break
	}
	if !errors.Is(gotErr, sentinel) {
		t.Errorf("got %v, want %v", gotErr, sentinel)
	}
}
