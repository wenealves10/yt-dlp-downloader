package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/server"
	"github.com/wenealves10/yt-dlp-downloader/pkg/sse"
)

func main() {
	config, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, config.DBSource)
	if err != nil {
		log.Fatal("Erro ao conectar no banco:", err)
	}
	defer pool.Close()

	store := db.NewStore(pool)

	redisAddr := fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: config.RedisUsername,
		Password: config.RedisPassword,
	})
	defer asynqClient.Close()

	// Initialize the Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: config.RedisUsername,
		Password: config.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	rdStream := stream.NewRedisConsumer(rdb, stream.ConsumerGroup, stream.ConsumerName, stream.StreamName)
	sseManager := sse.NewSSEManager()

	var chDownloads = make(chan string, 100)
	go rdStream.Consume(ctx, chDownloads)

	// Start a goroutine to handle SSE events
	go func() {
		for msg := range chDownloads {
			var event stream.DownloadEvent
			err := json.Unmarshal([]byte(msg), &event)
			if err != nil {
				log.Printf("Error unmarshalling SSE message: %v", err)
				continue
			}
			sseManager.Publish(event.UserID, msg)
		}
	}()

	api, err := server.NewServer(config, store, asynqClient, sseManager)
	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	if err := api.Start(config.ServerAddress); err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
