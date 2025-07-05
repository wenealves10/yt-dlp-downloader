package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

type JobDeleteDownload struct {
	client    *asynq.Client
	r2Storage storage.Storage
	store     db.Store
}

func NewJobDeleteDownload(client *asynq.Client, r2Storage storage.Storage, store db.Store) *JobDeleteDownload {
	return &JobDeleteDownload{
		client:    client,
		r2Storage: r2Storage,
		store:     store,
	}
}

func (p *JobDeleteDownload) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.DeleteDownloadPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	downloadID, err := utils.ParseUUID(payload.DownloadID)
	if err != nil {
		return fmt.Errorf("invalid download ID format: %v", err)
	}

	downloadExists, err := p.store.GetDownloadByID(ctx, downloadID)
	if err != nil {
		return fmt.Errorf("failed to check download existence: %v", err)
	}

	if err := p.store.DeleteDownload(ctx, downloadExists.ID); err != nil {
		return fmt.Errorf("failed to delete download from database: %v", err)
	}

	if err := p.r2Storage.DeleteFile(ctx, downloadExists.ThumbnailUrl.String); err != nil {
		log.Printf("Failed to delete thumbnail from storage: %v", err)
		return fmt.Errorf("failed to delete thumbnail from storage: %v", err)
	}

	if err := p.r2Storage.DeleteFile(ctx, downloadExists.FileUrl.String); err != nil {
		log.Printf("Failed to delete file from storage: %v", err)
		return fmt.Errorf("failed to delete file from storage: %v", err)
	}

	return nil
}
