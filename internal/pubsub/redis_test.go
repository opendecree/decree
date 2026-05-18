package pubsub

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var discardLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))

// Tests below use nil Redis clients. Only the constructor and Close paths are
// exercised — no actual Redis calls are made.

func TestNewRedisPublisher(t *testing.T) {
	p := NewRedisPublisher(nil)
	require.NotNil(t, p)
}

func TestRedisPublisher_Close(t *testing.T) {
	p := NewRedisPublisher(nil)
	require.NoError(t, p.Close())
}

func TestNewRedisSubscriber(t *testing.T) {
	s := NewRedisSubscriber(nil, discardLogger)
	require.NotNil(t, s)
}

func TestRedisSubscriber_Close(t *testing.T) {
	s := NewRedisSubscriber(nil, discardLogger)
	require.NoError(t, s.Close())
}
