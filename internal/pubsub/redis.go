package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const channelPrefix = "configchange:"

// RedisPublisher implements Publisher using Redis Pub/Sub.
type RedisPublisher struct {
	client *redis.Client
}

// NewRedisPublisher creates a new Redis-backed publisher.
func NewRedisPublisher(client *redis.Client) *RedisPublisher {
	return &RedisPublisher{client: client}
}

func (p *RedisPublisher) Publish(ctx context.Context, event ConfigChangeEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
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

	go func() {
		defer close(ch)
		defer func() { _ = sub.Close() }()

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
