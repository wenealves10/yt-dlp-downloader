package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage/r2"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/server"
	"github.com/wenealves10/yt-dlp-downloader/pkg/sse"
)

func main() {
	cg, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cg.DBSource)
	if err != nil {
		log.Fatal("Erro ao conectar no banco:", err)
	}
	defer pool.Close()
	store := db.NewStore(pool)

	accessKeyId := cg.AccessKeyID
	accessKeySecret := cg.SecretAccessKey
	region := cg.Region
	endpoint := cg.EndpointURL
	bucketName := cg.BucketName

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion(region),
	)
	if err != nil {
		log.Fatal(err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	r2Storage := r2.NewS3Service(s3Client, bucketName)

	redisAddr := fmt.Sprintf("%s:%s", cg.RedisHost, cg.RedisPort)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: cg.RedisUsername,
		Password: cg.RedisPassword,
	})
	defer asynqClient.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: cg.RedisUsername,
		Password: cg.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	rdStream := stream.NewRedisConsumer(rdb, stream.ConsumerGroup, stream.ConsumerName, stream.StreamName)
	sseManager := sse.NewSSEManager()

	var chDownloads = make(chan string, 100)
	go rdStream.Consume(ctx, chDownloads)

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

	api, err := server.NewServer(cg, store, asynqClient, sseManager, r2Storage)
	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	if err := api.Start(cg.ServerAddress); err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
