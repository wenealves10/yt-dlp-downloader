package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

type JobUploadVideo struct {
	client    *asynq.Client
	r2Storage storage.Storage
	store     db.Store
	rdStream  stream.EventPublisher
}

func NewJobUploadVideo(client *asynq.Client, r2Storage storage.Storage, store db.Store, rdStream stream.EventPublisher) *JobUploadVideo {
	return &JobUploadVideo{
		client:    client,
		r2Storage: r2Storage,
		store:     store,
		rdStream:  rdStream,
	}
}

func (p *JobUploadVideo) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.UploadVideoPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	videoPathDest := fmt.Sprintf("uploads/videos/%s", payload.Filename)
	bannerPathDest := fmt.Sprintf("uploads/banners/%s_banner.jpg", utils.GenerateUUIDString())

	// Check if the download ID exists in the database
	downloadID, err := utils.ParseUUID(payload.DownloadID)
	if err != nil {
		return fmt.Errorf("invalid download ID format: %v", err)
	}

	downloadExists, err := p.store.GetDownloadByID(ctx, downloadID)
	if err != nil {
		return fmt.Errorf("failed to check download existence: %v: %w", err, asynq.SkipRetry)
	}

	if err := p.r2Storage.UploadFile(ctx, payload.VideoPath, videoPathDest); err != nil {
		return fmt.Errorf("failed to upload video file: %v: %w", err, asynq.SkipRetry)
	}

	if err := p.r2Storage.UploadFile(ctx, payload.BannerPath, bannerPathDest); err != nil {
		return fmt.Errorf("failed to upload banner file: %v: %w", err, asynq.SkipRetry)
	}

	expiredAt := time.Now().Add(24 * time.Hour)

	// atualizar o download no banco de dados
	if err := p.store.UpdateDownload(ctx, db.UpdateDownloadParams{
		ID:           downloadID,
		Status:       db.CoreDownloadStatusCOMPLETED,
		ThumbnailUrl: pgtype.Text{String: bannerPathDest, Valid: true},
		FileUrl:      pgtype.Text{String: videoPathDest, Valid: true},
		ExpiresAt:    pgtype.Timestamptz{Time: expiredAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to update download: %v: %w", err, asynq.SkipRetry)
	}

	// Publish the event to the Redis stream
	if err := p.rdStream.Publish(ctx, stream.DownloadEvent{
		ID:           payload.DownloadID,
		Status:       db.CoreDownloadStatusCOMPLETED,
		FileUrl:      videoPathDest,
		UserID:       downloadExists.UserID.String(),
		ThumbnailUrl: bannerPathDest,
		ExpiresAt:    expiredAt,
	}); err != nil {
		log.Println("failed to publish download event:", err)
	}

	// Optionally, you can remove the local video file after uploading
	if err := utils.RemoveFile(payload.VideoPath); err != nil {
		return fmt.Errorf("failed to remove local video file: %v: %w", err, asynq.SkipRetry)
	}

	if err := utils.RemoveFile(payload.BannerPath); err != nil {
		return fmt.Errorf("failed to remove local banner file: %v: %w", err, asynq.SkipRetry)
	}

	return nil
}
