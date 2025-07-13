package r2

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
)

type s3Service struct {
	client     *s3.Client
	bucketName string
}

func NewS3Service(client *s3.Client, bucket string) storage.Storage {
	return &s3Service{
		client:     client,
		bucketName: bucket,
	}
}

func (s *s3Service) UploadFile(ctx context.Context, filePath, objectKey string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	uploader := manager.NewUploader(s.client)

	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	fmt.Printf("File %s uploaded to bucket %s with key %s\n", filePath, objectKey, s.bucketName)
	return nil
}

func (s *s3Service) UploadFileByte(ctx context.Context, fileData []byte, objectKey string) error {
	uploader := manager.NewUploader(s.client)
	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(fileData),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file bytes: %w", err)
	}
	fmt.Printf("File bytes uploaded to bucket %s with key %s\n", s.bucketName, objectKey)
	return nil
}

func (s *s3Service) DownloadFile(ctx context.Context, objectKey, downloadPath string) error {
	file, err := os.Create(downloadPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	downloader := manager.NewDownloader(s.client)

	_, err = downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	fmt.Printf("File %s downloaded from bucket %s with key %s\n", downloadPath, objectKey, s.bucketName)
	return nil
}

func (s *s3Service) DeleteFile(ctx context.Context, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	fmt.Printf("File with key %s deleted from bucket %s\n", objectKey, s.bucketName)
	return nil
}
