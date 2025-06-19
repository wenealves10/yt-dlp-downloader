package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type EventPublisher interface {
	Publish(ctx context.Context, event DownloadEvent) error
}

type RedisPublisher struct {
	client *redis.Client
}

func NewRedisPublisher(client *redis.Client) EventPublisher {
	return &RedisPublisher{client: client}
}

func (r *RedisPublisher) Publish(ctx context.Context, event DownloadEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("erro ao serializar evento: %w", err)
	}

	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		Values: map[string]any{
			"payload": data,
		},
	}).Err()
}
