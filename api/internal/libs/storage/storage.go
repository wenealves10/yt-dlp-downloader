package storage

import (
	"context"
	"io"
)

// FileStream is an object read directly from the backing storage. Callers must
// close Body when they have finished sending it to the client.
type FileStream struct {
	Body          io.ReadCloser
	ContentLength int64
	ContentType   string
}

type Storage interface {
	UploadFile(ctx context.Context, filePath, objectKey string) error
	UploadFileByte(ctx context.Context, fileData []byte, objectKey string) error
	DownloadFile(ctx context.Context, objectKey, downloadPath string) error
	OpenFile(ctx context.Context, objectKey string) (FileStream, error)
	DeleteFile(ctx context.Context, objectKey string) error
}
