// Package pgconv provides conversion helpers between domain types and
// PostgreSQL-specific types (pgtype, pgx). Only imported by PG store
// implementations — never by service layer code.
package pgconv

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/opendecree/decree/internal/storage/domain"
)

// StringToUUID converts a string UUID to pgtype.UUID.
func StringToUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return id, nil
}

// MustUUID converts a string UUID to pgtype.UUID, panicking on invalid input.
// Use only when the UUID has already been validated.
func MustUUID(s string) pgtype.UUID {
	id, err := StringToUUID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// UUIDToString converts a pgtype.UUID to a string.
// Returns empty string for invalid/null UUIDs.
func UUIDToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

// TimeToTimestamptz converts a time.Time to pgtype.Timestamptz.
func TimeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// OptionalTimeToTimestamptz converts a *time.Time to pgtype.Timestamptz.
// Returns an invalid Timestamptz for nil.
func OptionalTimeToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// TimestamptzToTime converts a pgtype.Timestamptz to time.Time.
// Returns zero time for invalid/null timestamps.
func TimestamptzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

// TimestamptzToOptionalTime converts a pgtype.Timestamptz to *time.Time.
// Returns nil for invalid/null timestamps.
func TimestamptzToOptionalTime(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	return &ts.Time
}

// WrapNotFound converts pgx.ErrNoRows to domain.ErrNotFound.
// Returns the original error for all other errors.
func WrapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// FieldTypeToDB converts a domain FieldType to the DB string representation.
func FieldTypeToDB(ft domain.FieldType) string {
	return string(ft)
}

// FieldTypeFromDB converts a DB field type string to domain FieldType.
func FieldTypeFromDB(s string) domain.FieldType {
	return domain.FieldType(s)
}
