package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type EventPublisher interface {
	Publish(ctx context.Context, streamName string, payload any) error
}

type RedisPublisher struct {
	client *redis.Client
}

func NewRedisPublisher(client *redis.Client) EventPublisher {
	return &RedisPublisher{client: client}
}

func (r *RedisPublisher) Publish(ctx context.Context, streamName string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao serializar evento: %w", err)
	}

	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]any{
			"payload": data,
		},
	}).Err()
}
