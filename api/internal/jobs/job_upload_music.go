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

type JobUploadMusic struct {
	client    *asynq.Client
	r2Storage storage.Storage
}

func NewJobUploadMusic(client *asynq.Client, r2Storage storage.Storage) *JobUploadMusic {
	return &JobUploadMusic{
		client:    client,
		r2Storage: r2Storage,
	}
}

func (p *JobUploadMusic) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.UploadMusicPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	musicPathDest := fmt.Sprintf("uploads/musics/%s", payload.Filename)

	if err := p.r2Storage.UploadFile(ctx, payload.MusicPath, musicPathDest); err != nil {
		return fmt.Errorf("failed to upload music file: %v: %w", err, asynq.SkipRetry)
	}

	// Optionally, you can remove the local music file after uploading
	if err := utils.RemoveFile(payload.MusicPath); err != nil {
		return fmt.Errorf("failed to remove local video file: %v: %w", err, asynq.SkipRetry)
	}

	return nil
}
