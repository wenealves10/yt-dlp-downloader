package stream

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type EventConsumer interface {
	Consume(ctx context.Context, ch chan<- DownloadEvent)
}

type RedisConsumer struct {
	client   *redis.Client
	group    string
	consumer string
}

func NewRedisConsumer(client *redis.Client, group, consumer string) EventConsumer {
	return &RedisConsumer{client, group, consumer}
}

func (r *RedisConsumer) Consume(ctx context.Context, ch chan<- DownloadEvent) {
	for {
		streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    r.group,
			Consumer: r.consumer,
			Streams:  []string{StreamName, ">"},
			Block:    5 * time.Second,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue
			}
			log.Println("erro lendo do stream:", err)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				raw := msg.Values["payload"].(string)
				var event DownloadEvent
				if err := json.Unmarshal([]byte(raw), &event); err != nil {
					log.Println("erro ao desserializar evento:", err)
					continue
				}
				ch <- event

				// Marca como processado
				r.client.XAck(ctx, StreamName, r.group, msg.ID)
			}
		}
	}
}
