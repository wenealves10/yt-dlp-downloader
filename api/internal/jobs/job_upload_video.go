package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

type JobUploadVideo struct {
	client    *asynq.Client
	r2Storage storage.Storage
}

func NewJobUploadVideo(client *asynq.Client, r2Storage storage.Storage) *JobUploadVideo {
	return &JobUploadVideo{
		client:    client,
		r2Storage: r2Storage,
	}
}

func (p *JobUploadVideo) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.UploadVideoPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	videoPathDest := fmt.Sprintf("uploads/videos/%s", payload.Filename)

	if err := p.r2Storage.UploadFile(ctx, payload.VideoPath, videoPathDest); err != nil {
		return fmt.Errorf("failed to upload video file: %v: %w", err, asynq.SkipRetry)
	}

	// Optionally, you can remove the local video file after uploading
	if err := utils.RemoveFile(payload.VideoPath); err != nil {
		return fmt.Errorf("failed to remove local video file: %v: %w", err, asynq.SkipRetry)
	}

	return nil
}
