package r2

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
)

type s3Service struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
}

func NewS3Service(client *s3.Client, bucket string) storage.Storage {
	return &s3Service{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucketName:    bucket,
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

// OpenFile returns the R2 object body without writing it to local disk. This
// lets the API proxy the file to the browser as a download.
func (s *s3Service) OpenFile(ctx context.Context, objectKey string) (storage.FileStream, error) {
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return storage.FileStream{}, fmt.Errorf("failed to open file: %w", err)
	}

	contentLength := int64(-1)
	if object.ContentLength != nil {
		contentLength = *object.ContentLength
	}

	return storage.FileStream{
		Body:          object.Body,
		ContentLength: contentLength,
		ContentType:   aws.ToString(object.ContentType),
	}, nil
}

// PresignDownload creates a GET URL authenticated by R2's S3 signature. It is
// deliberately short lived and includes the attachment headers in the signed
// request, so a caller cannot change the filename or force an inline response.
func (s *s3Service) PresignDownload(ctx context.Context, objectKey, filename, contentType string, expiresIn time.Duration) (string, error) {
	contentDisposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	input := &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucketName),
		Key:                        aws.String(objectKey),
		ResponseContentDisposition: aws.String(contentDisposition),
	}
	if contentType != "" {
		input.ResponseContentType = aws.String(contentType)
	}

	presigned, err := s.presignClient.PresignGetObject(ctx, input, func(options *s3.PresignOptions) {
		options.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign download: %w", err)
	}

	return presigned.URL, nil
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
