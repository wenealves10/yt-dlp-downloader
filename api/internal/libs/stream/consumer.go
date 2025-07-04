package stream

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type EventConsumer interface {
	Consume(ctx context.Context, ch chan<- string)
}

type RedisConsumer struct {
	client     *redis.Client
	group      string
	consumer   string
	streamName string
}

func NewRedisConsumer(client *redis.Client, group, consumer, streamName string) EventConsumer {
	err := client.XGroupCreateMkStream(context.Background(), streamName, group, "$").Err()
	if err != nil && err != redis.Nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		log.Fatalf("erro ao criar grupo de consumo: %v", err)
	}

	return &RedisConsumer{client, group, consumer, streamName}
}

func (r *RedisConsumer) Consume(ctx context.Context, ch chan<- string) {
	for {
		streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    r.group,
			Consumer: r.consumer,
			Streams:  []string{r.streamName, ">"},
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
				ch <- raw
				// Marca como processado
				r.client.XAck(ctx, StreamName, r.group, msg.ID)
			}
		}
	}
}
