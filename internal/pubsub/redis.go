package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

const channelPrefix = "configchange:"

// newEventID returns a random UUID v4 string.
func newEventID() string {
	var b [16]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// RedisPublisher implements Publisher using Redis Pub/Sub.
//
// Delivery is at-most-once: if the Redis connection drops during PUBLISH the
// event is lost with no retry.
type RedisPublisher struct {
	client *redis.Client
	seq    atomic.Int64
}

// NewRedisPublisher creates a new Redis-backed publisher.
func NewRedisPublisher(client *redis.Client) *RedisPublisher {
	return &RedisPublisher{client: client}
}

func (p *RedisPublisher) Publish(ctx context.Context, event ConfigChangeEvent) error {
	event.EventID = newEventID()
	event.Seq = p.seq.Add(1)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if len(data) > MaxPayloadBytes {
		return ErrPayloadTooLarge
	}
	channel := channelPrefix + event.TenantID
	if err := p.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("redis publish: %w", err)
	}
	return nil
}

func (p *RedisPublisher) Close() error {
	return nil
}

// RedisSubscriber implements Subscriber using Redis Pub/Sub.
//
// Delivery is at-most-once: a disconnected subscriber misses events published
// during the outage with no replay mechanism.
type RedisSubscriber struct {
	client *redis.Client
	logger *slog.Logger
}

// NewRedisSubscriber creates a new Redis-backed subscriber.
func NewRedisSubscriber(client *redis.Client, logger *slog.Logger) *RedisSubscriber {
	return &RedisSubscriber{client: client, logger: logger}
}

func (s *RedisSubscriber) Subscribe(ctx context.Context, tenantID string) (<-chan ConfigChangeEvent, context.CancelFunc, error) {
	channel := channelPrefix + tenantID
	sub := s.client.Subscribe(ctx, channel)

	// Verify subscription is active.
	if _, err := sub.Receive(ctx); err != nil {
		return nil, nil, fmt.Errorf("redis subscribe: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	ch := make(chan ConfigChangeEvent, 64)

	// Close the Redis subscription when subCtx is cancelled. This causes
	// sub.Channel() to be closed, which unblocks the goroutine below without
	// relying solely on the Done channel select case.
	context.AfterFunc(subCtx, func() { _ = sub.Close() })

	go func() {
		defer close(ch)

		msgCh := sub.Channel()
		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				var event ConfigChangeEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					s.logger.ErrorContext(subCtx, "unmarshal config change event", "error", err)
					continue
				}
				select {
				case ch <- event:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return ch, cancel, nil
}

func (s *RedisSubscriber) Close() error {
	return nil
}
