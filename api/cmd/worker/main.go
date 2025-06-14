package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/jobs"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage/r2"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
)

func main() {

	accessKeyId := configs.ACCESS_KEY_ID
	accessKeySecret := configs.SECRET_ACCESS_KEY
	region := configs.REGION
	endpoint := configs.ENDPOINT_URL
	bucketName := configs.BUCKET_NAME

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

	redisAddr := fmt.Sprintf("%s:%s", configs.REDIS_HOST, configs.REDIS_PORT)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Username: configs.REDIS_USERNAME,
			Password: configs.REDIS_PASSWORD,
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
		Username: configs.REDIS_USERNAME,
		Password: configs.REDIS_PASSWORD,
	})
	defer asynqClient.Close()

	mux := asynq.NewServeMux()
	mux.Handle(tasks.TypeDownloadVideo, jobs.NewJobDownloadVideo(asynqClient))
	mux.Handle(tasks.TypeDownloadMusic, jobs.NewJobDownloadMusic(asynqClient))
	mux.Handle(tasks.TypeUploadVideo, jobs.NewJobUploadVideo(asynqClient, r2Storage))
	mux.Handle(tasks.TypeUploadMusic, jobs.NewJobUploadMusic(asynqClient, r2Storage))
	log.Printf("Starting worker server on %s", redisAddr)

	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
