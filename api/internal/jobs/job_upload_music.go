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

type JobUploadMusic struct {
	client    *asynq.Client
	r2Storage storage.Storage
	store     db.Store
	rdStream  stream.EventPublisher
}

func NewJobUploadMusic(client *asynq.Client, r2Storage storage.Storage, store db.Store, rdStream stream.EventPublisher) *JobUploadMusic {
	return &JobUploadMusic{
		client:    client,
		r2Storage: r2Storage,
		store:     store,
		rdStream:  rdStream,
	}
}

func (p *JobUploadMusic) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.UploadMusicPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	musicPathDest := fmt.Sprintf("uploads/musics/%s", payload.Filename)
	bannerPathDest := fmt.Sprintf("uploads/banners/%s_banner.jpg", utils.GenerateUUIDString())

	downloadID, err := utils.ParseUUID(payload.DownloadID)
	if err != nil {
		return fmt.Errorf("invalid download ID format: %v", err)
	}

	downloadExists, err := p.store.GetDownloadByID(ctx, downloadID)
	if err != nil {
		return fmt.Errorf("failed to check download existence: %v: %w", err, asynq.SkipRetry)
	}

	// Medido antes do upload, porque o arquivo local é apagado logo em
	// seguida. É o que alimenta as métricas de armazenamento do painel; falhar
	// aqui não pode derrubar um download já concluído.
	fileSize, err := utils.FileSize(payload.MusicPath)
	if err != nil {
		log.Printf("failed to read file size for download %s: %v", payload.DownloadID, err)
	}

	if err := p.r2Storage.UploadFile(ctx, payload.MusicPath, musicPathDest); err != nil {
		return fmt.Errorf("failed to upload music file: %v: %w", err, asynq.SkipRetry)
	}

	if err := p.r2Storage.UploadFile(ctx, payload.BannerPath, bannerPathDest); err != nil {
		return fmt.Errorf("failed to upload banner file: %v: %w", err,
			asynq.SkipRetry)
	}

	expiredAt := time.Now().Add(24 * time.Hour)
	if err := p.store.UpdateDownload(ctx, db.UpdateDownloadParams{
		ID:           downloadID,
		Status:       db.CoreDownloadStatusCOMPLETED,
		ThumbnailUrl: pgtype.Text{String: bannerPathDest, Valid: true},
		FileUrl:      pgtype.Text{String: musicPathDest, Valid: true},
		ExpiresAt:    pgtype.Timestamptz{Time: expiredAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to update download: %v: %w", err, asynq.SkipRetry)
	}

	if fileSize > 0 {
		if err := p.store.SetDownloadFileSize(ctx, db.SetDownloadFileSizeParams{
			ID:            downloadID,
			FileSizeBytes: fileSize,
		}); err != nil {
			log.Printf("failed to record file size for download %s: %v", payload.DownloadID, err)
		}
	}

	if err := p.rdStream.Publish(ctx, stream.StreamName, stream.DownloadEvent{
		ID:           payload.DownloadID,
		UserID:       downloadExists.UserID.String(),
		Status:       db.CoreDownloadStatusCOMPLETED,
		FileUrl:      musicPathDest,
		ThumbnailUrl: bannerPathDest,
		ExpiresAt:    &expiredAt,
	}); err != nil {
		log.Println("failed to publish download event:", err)
	}

	if err := utils.RemoveFile(payload.MusicPath); err != nil {
		return fmt.Errorf("failed to remove local video file: %v: %w", err, asynq.SkipRetry)
	}

	if err := utils.RemoveFile(payload.BannerPath); err != nil {
		return fmt.Errorf("failed to remove local banner file: %v: %w", err, asynq.SkipRetry)
	}

	return nil
}
