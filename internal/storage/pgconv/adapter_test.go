package pgconv

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

// TestConverter verifies that Converter delegates correctly to the package-level
// functions. One call per method is sufficient — correctness of the underlying
// functions is already covered by convert_test.go.
func TestConverter(t *testing.T) {
	c := Converter{}
	uuid := "11111111-1111-1111-1111-111111111111"

	t.Run("StringToUUID valid", func(t *testing.T) {
		id, err := c.StringToUUID(uuid)
		require.NoError(t, err)
		assert.True(t, id.Valid)
	})

	t.Run("MustUUID", func(t *testing.T) {
		id := c.MustUUID(uuid)
		assert.True(t, id.Valid)
	})

	t.Run("UUIDToString round-trip", func(t *testing.T) {
		id, _ := c.StringToUUID(uuid)
		assert.Equal(t, uuid, c.UUIDToString(id))
	})

	t.Run("TimeToTimestamptz round-trip", func(t *testing.T) {
		now := time.Now().Truncate(time.Microsecond).UTC()
		ts := c.TimeToTimestamptz(now)
		assert.True(t, ts.Valid)
		assert.Equal(t, now, c.TimestamptzToTime(ts))
	})

	t.Run("OptionalTimeToTimestamptz nil", func(t *testing.T) {
		ts := c.OptionalTimeToTimestamptz(nil)
		assert.False(t, ts.Valid)
	})

	t.Run("OptionalTimeToTimestamptz non-nil", func(t *testing.T) {
		now := time.Now().Truncate(time.Microsecond).UTC()
		ts := c.OptionalTimeToTimestamptz(&now)
		assert.True(t, ts.Valid)
		got := c.TimestamptzToOptionalTime(ts)
		require.NotNil(t, got)
		assert.Equal(t, now, *got)
	})

	t.Run("TimestamptzToOptionalTime invalid", func(t *testing.T) {
		got := c.TimestamptzToOptionalTime(pgtype.Timestamptz{})
		assert.Nil(t, got)
	})

	t.Run("WrapNotFound passthrough", func(t *testing.T) {
		base := errors.New("base")
		err := c.WrapNotFound(base)
		assert.Same(t, base, err)
	})

	t.Run("WrapUniqueViolation passthrough", func(t *testing.T) {
		base := errors.New("base")
		err := c.WrapUniqueViolation(base)
		assert.Same(t, base, err)
	})

	t.Run("WrapFKViolation passthrough", func(t *testing.T) {
		base := errors.New("base")
		err := c.WrapFKViolation(base)
		assert.Same(t, base, err)
	})

	t.Run("FieldTypeToDB round-trip", func(t *testing.T) {
		ft := domain.FieldTypeString
		s := c.FieldTypeToDB(ft)
		assert.NotEmpty(t, s)
		assert.Equal(t, ft, c.FieldTypeFromDB(s))
	})
}
