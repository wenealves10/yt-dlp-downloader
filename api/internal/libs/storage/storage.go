package storage

import "context"

type Storage interface {
	UploadFile(ctx context.Context, filePath, objectKey string) error
	UploadFileByte(ctx context.Context, fileData []byte, objectKey string) error
	DownloadFile(ctx context.Context, objectKey, downloadPath string) error
	DeleteFile(ctx context.Context, objectKey string) error
}
