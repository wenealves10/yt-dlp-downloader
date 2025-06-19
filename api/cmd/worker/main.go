package main

import (
	"context"
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
	"github.com/wenealves10/yt-dlp-downloader/internal/jobs"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage/r2"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
)

func main() {
	cg, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

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

	// Create an S3 client with the custom endpoint for Cloudflare R2
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	r2Storage := r2.NewS3Service(s3Client, bucketName)

	// Initialize the Database connection
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cg.DBSource)
	if err != nil {
		log.Fatal("Erro ao conectar no banco:", err)
	}
	defer pool.Close()
	store := db.NewStore(pool)

	redisAddr := fmt.Sprintf("%s:%s", cg.RedisHost, cg.RedisPort)
	// Initialize the Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: cg.RedisUsername,
		Password: cg.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	rdStream := stream.NewRedisPublisher(rdb)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Username: cg.RedisUsername,
			Password: cg.RedisPassword,
		},
		asynq.Config{
			Concurrency: queues.Concurrency,
			Queues: map[string]int{
				queues.TypeDownloadVideoQueue: queues.ConcurrencyDownloadVideo,
				queues.TypeDownloadMusicQueue: queues.ConcurrencyDownloadMusic,
				queues.TypeUploadVideoQueue:   queues.ConcurrencyUploadVideo,
				queues.TypeUploadMusicQueue:   queues.ConcurrencyUploadMusic,
			},
		},
	)

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: cg.RedisUsername,
		Password: cg.RedisPassword,
	})
	defer asynqClient.Close()

	mux := asynq.NewServeMux()
	mux.Handle(tasks.TypeDownloadVideo, jobs.NewJobDownloadVideo(asynqClient, store, rdStream))
	mux.Handle(tasks.TypeDownloadMusic, jobs.NewJobDownloadMusic(asynqClient, store, rdStream))
	mux.Handle(tasks.TypeUploadVideo, jobs.NewJobUploadVideo(asynqClient, r2Storage, store, rdStream))
	mux.Handle(tasks.TypeUploadMusic, jobs.NewJobUploadMusic(asynqClient, r2Storage, store, rdStream))
	log.Printf("Starting worker server on %s", redisAddr)

	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
