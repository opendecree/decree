// Package pagination provides opaque page-token encoding for list RPCs.
//
// Tokens are versioned to allow future migration from offset-based to
// keyset-based pagination without breaking existing clients.
package pagination

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const tokenVersion = "v1"

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
