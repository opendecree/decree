package pgconv

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/opendecree/decree/internal/storage/domain"
)

// Converter is a named adapter type that groups the pgconv conversion helpers
// as methods. It is zero-value ready and carries no state; use it where a
// struct dependency is more convenient than calling the package-level
// functions directly (e.g. to inject a test double or satisfy an interface).
//
// All methods delegate to the corresponding package-level function so
// behaviour is identical.
type Converter struct{}

func (Converter) StringToUUID(s string) (pgtype.UUID, error) { return StringToUUID(s) }
func (Converter) MustUUID(s string) pgtype.UUID              { return MustUUID(s) }
func (Converter) UUIDToString(id pgtype.UUID) string         { return UUIDToString(id) }

func (Converter) TimeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return TimeToTimestamptz(t)
}

func (Converter) OptionalTimeToTimestamptz(t *time.Time) pgtype.Timestamptz {
	return OptionalTimeToTimestamptz(t)
}

func (Converter) TimestamptzToTime(ts pgtype.Timestamptz) time.Time {
	return TimestamptzToTime(ts)
}

func (Converter) TimestamptzToOptionalTime(ts pgtype.Timestamptz) *time.Time {
	return TimestamptzToOptionalTime(ts)
}

func (Converter) WrapNotFound(err error) error        { return WrapNotFound(err) }
func (Converter) WrapUniqueViolation(err error) error { return WrapUniqueViolation(err) }
func (Converter) WrapFKViolation(err error) error     { return WrapFKViolation(err) }

func (Converter) FieldTypeToDB(ft domain.FieldType) string  { return FieldTypeToDB(ft) }
func (Converter) FieldTypeFromDB(s string) domain.FieldType { return FieldTypeFromDB(s) }
