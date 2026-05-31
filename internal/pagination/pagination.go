// Package pagination provides opaque page-token encoding for list RPCs.
//
// Two token formats are supported:
//   - v1: offset-based (legacy, backward-compat)
//   - v2: keyset cursor encoding (created_at unix-ns + entry UUID)
package pagination

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	tokenVersion       = "v1"
	cursorTokenVersion = "v2"
)

// PageCursor is a keyset pagination cursor for queries ordered by (created_at DESC, id DESC).
type PageCursor struct {
	Time time.Time
	ID   string
}

// TokenKind indicates how a decoded page token encodes the pagination position.
type TokenKind uint8

const (
	KindFirst  TokenKind = iota // empty token: first page, use keyset with nil cursor
	KindOffset                  // v1 token: legacy offset-based (backward compat)
	KindCursor                  // v2 token: keyset cursor
)

// DecodeTokenKind decodes a page token and returns its kind plus the decoded value.
// Returns (KindFirst, 0, PageCursor{}, nil) for an empty token.
func DecodeTokenKind(token string) (kind TokenKind, offset int32, cursor PageCursor, err error) {
	if token == "" {
		return KindFirst, 0, PageCursor{}, nil
	}
	raw, decErr := base64.StdEncoding.DecodeString(token)
	if decErr != nil {
		return 0, 0, PageCursor{}, fmt.Errorf("invalid page token")
	}
	rawStr := string(raw)
	switch {
	case strings.HasPrefix(rawStr, tokenVersion+":"):
		payload := rawStr[len(tokenVersion)+1:]
		offsetVal, parseErr := strconv.ParseInt(payload, 10, 32)
		if parseErr != nil || offsetVal < 0 {
			return 0, 0, PageCursor{}, fmt.Errorf("invalid page token")
		}
		return KindOffset, int32(offsetVal), PageCursor{}, nil
	case strings.HasPrefix(rawStr, cursorTokenVersion+":"):
		cur, parseErr := decodeCursorPayload(rawStr)
		if parseErr != nil {
			return 0, 0, PageCursor{}, parseErr
		}
		return KindCursor, 0, cur, nil
	default:
		return 0, 0, PageCursor{}, fmt.Errorf("invalid page token")
	}
}

func decodeCursorPayload(rawStr string) (PageCursor, error) {
	parts := strings.SplitN(rawStr, ":", 3)
	if len(parts) != 3 || parts[0] != cursorTokenVersion || parts[2] == "" {
		return PageCursor{}, fmt.Errorf("invalid page token")
	}
	ns, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return PageCursor{}, fmt.Errorf("invalid page token")
	}
	return PageCursor{Time: time.Unix(0, ns).UTC(), ID: parts[2]}, nil
}

// EncodeCursorToken encodes a keyset cursor into an opaque page token.
func EncodeCursorToken(cursor PageCursor) string {
	raw := fmt.Sprintf("%s:%d:%s", cursorTokenVersion, cursor.Time.UnixNano(), cursor.ID)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// NextCursorToken returns the cursor token for the next keyset page.
// lastTime and lastID come from entries[pageSize-1] after fetching pageSize+1 rows.
// Returns empty string if there are no more results.
func NextCursorToken(pageSize, resultCount int32, lastTime time.Time, lastID string) string {
	if resultCount <= pageSize {
		return ""
	}
	return EncodeCursorToken(PageCursor{Time: lastTime, ID: lastID})
}

// ClampPageSize returns a page size within [1, maxSize], defaulting to
// defaultSize when the requested size is zero or negative.
func ClampPageSize(requested, defaultSize, maxSize int32) int32 {
	if requested <= 0 {
		return defaultSize
	}
	if requested > maxSize {
		return maxSize
	}
	return requested
}

// DecodePageToken decodes an opaque page token into an offset.
// An empty token returns offset 0 (first page).
func DecodePageToken(token string) (int32, error) {
	if token == "" {
		return 0, nil
	}
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, fmt.Errorf("invalid page token")
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 || parts[0] != tokenVersion {
		return 0, fmt.Errorf("invalid page token")
	}
	offset, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid page token")
	}
	if offset < 0 {
		return 0, fmt.Errorf("invalid page token")
	}
	return int32(offset), nil
}

// EncodePageToken encodes an offset into an opaque page token.
func EncodePageToken(offset int32) string {
	raw := fmt.Sprintf("%s:%d", tokenVersion, offset)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// NextPageToken returns the token for the next page, or empty string if
// there are no more results. Call this after fetching pageSize+1 rows
// from the database.
func NextPageToken(pageSize, resultCount, currentOffset int32) string {
	if resultCount <= pageSize {
		return ""
	}
	return EncodePageToken(currentOffset + pageSize)
}
