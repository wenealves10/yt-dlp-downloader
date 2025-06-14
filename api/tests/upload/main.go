package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage/r2"
)

func main() {

	accessKeyId := configs.ACCESS_KEY_ID
	accessKeySecret := configs.SECRET_ACCESS_KEY
	region := configs.REGION
	bucketName := configs.BUCKET_NAME
	endpoint := configs.ENDPOINT_URL

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion(region),
	)
	if err != nil {
		log.Fatal(err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	r2Storage := r2.NewS3Service(client, bucketName)

	if err := r2Storage.UploadFile(context.TODO(), "./tests/upload/text.txt", "uploads/text4.txt"); err != nil {
		log.Fatalf("failed to upload file: %v", err)
	}

	fmt.Println("File uploaded successfully")
}
