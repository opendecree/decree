package pgconv

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

func TestStringToUUID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		id, err := StringToUUID("11111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		assert.True(t, id.Valid)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := StringToUUID("not-a-uuid")
		assert.Error(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := StringToUUID("")
		assert.Error(t, err)
	})
}

func TestMustUUID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		id := MustUUID("11111111-1111-1111-1111-111111111111")
		assert.True(t, id.Valid)
	})

	t.Run("panics on invalid", func(t *testing.T) {
		assert.Panics(t, func() { MustUUID("bad") })
	})
}

func TestUUIDToString(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		id, _ := StringToUUID("11111111-1111-1111-1111-111111111111")
		assert.Equal(t, "11111111-1111-1111-1111-111111111111", UUIDToString(id))
	})

	t.Run("null", func(t *testing.T) {
		assert.Equal(t, "", UUIDToString(pgtype.UUID{}))
	})
}

func TestUUID_RoundTrip(t *testing.T) {
	original := "abcdef01-2345-6789-abcd-ef0123456789"
	id, err := StringToUUID(original)
	require.NoError(t, err)
	assert.Equal(t, original, UUIDToString(id))
}

func TestTimeToTimestamptz(t *testing.T) {
	now := time.Now()
	ts := TimeToTimestamptz(now)
	assert.True(t, ts.Valid)
	assert.Equal(t, now, ts.Time)
}

func TestOptionalTimeToTimestamptz(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		ts := OptionalTimeToTimestamptz(nil)
		assert.False(t, ts.Valid)
	})

	t.Run("non-nil", func(t *testing.T) {
		now := time.Now()
		ts := OptionalTimeToTimestamptz(&now)
		assert.True(t, ts.Valid)
		assert.Equal(t, now, ts.Time)
	})
}

func TestTimestamptzToTime(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		now := time.Now()
		ts := pgtype.Timestamptz{Time: now, Valid: true}
		assert.Equal(t, now, TimestamptzToTime(ts))
	})

	t.Run("invalid", func(t *testing.T) {
		assert.True(t, TimestamptzToTime(pgtype.Timestamptz{}).IsZero())
	})
}

func TestTimestamptzToOptionalTime(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		now := time.Now()
		ts := pgtype.Timestamptz{Time: now, Valid: true}
		result := TimestamptzToOptionalTime(ts)
		require.NotNil(t, result)
		assert.Equal(t, now, *result)
	})

	t.Run("invalid", func(t *testing.T) {
		assert.Nil(t, TimestamptzToOptionalTime(pgtype.Timestamptz{}))
	})
}

func TestWrapNotFound(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, WrapNotFound(nil))
	})

	t.Run("pgx.ErrNoRows", func(t *testing.T) {
		err := WrapNotFound(pgx.ErrNoRows)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("other error", func(t *testing.T) {
		other := errors.New("something else")
		assert.Equal(t, other, WrapNotFound(other))
	})
}

func TestFieldTypeConversions(t *testing.T) {
	types := []domain.FieldType{
		domain.FieldTypeInteger, domain.FieldTypeNumber, domain.FieldTypeString,
		domain.FieldTypeBool, domain.FieldTypeTime, domain.FieldTypeDuration,
		domain.FieldTypeURL, domain.FieldTypeJSON,
	}
	for _, ft := range types {
		assert.Equal(t, string(ft), FieldTypeToDB(ft))
		assert.Equal(t, ft, FieldTypeFromDB(string(ft)))
	}
}
