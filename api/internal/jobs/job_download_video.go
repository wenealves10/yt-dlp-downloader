package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

type JobDownloadVideo struct {
	client   *asynq.Client
	store    db.Store
	rdStream stream.EventPublisher
}

func NewJobDownloadVideo(client *asynq.Client, store db.Store, rdStream stream.EventPublisher) *JobDownloadVideo {
	return &JobDownloadVideo{
		client:   client,
		store:    store,
		rdStream: rdStream,
	}
}

func (p *JobDownloadVideo) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.DownloadVideoPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	filename := fmt.Sprintf("video_%s.mp4", uuid.New().String())
	outputPath := "./uploads/videos"

	// Check if the download ID exists in the database
	downloadID, err := utils.ParseUUID(payload.DownloadID)
	if err != nil {
		return fmt.Errorf("invalid download ID format: %v", err)
	}

	downloadExists, err := p.store.GetDownloadByID(ctx, downloadID)
	if err != nil {
		return fmt.Errorf("failed to check download existence: %v", err)
	}

	// atualiza o status do download para "in_progress"
	if err := p.store.UpdateDownloadStatus(ctx, db.UpdateDownloadStatusParams{
		ID:     downloadExists.ID,
		Status: db.CoreDownloadStatusPROCESSING,
	}); err != nil {
		return fmt.Errorf("failed to update download status: %v", err)
	}

	if err := p.rdStream.Publish(ctx, stream.DownloadEvent{
		ID:     downloadExists.ID.String(),
		UserID: downloadExists.UserID.String(),
		Status: db.CoreDownloadStatusPROCESSING,
	}); err != nil {
		log.Println("failed to publish download event:", err)
	}

	outputFilePath, err := p.downloadVideo(ctx, filename, outputPath, downloadExists.OriginalUrl)
	if err != nil {
		return err
	}

	//send image uploader task
	taskUpload, err := tasks.NewUploadVideoTask(outputFilePath, payload.DownloadID, filename)
	if err != nil {
		return fmt.Errorf("failed to create upload video task: %v", err)
	}

	// Enqueue the image uploader task
	if _, err := p.client.Enqueue(taskUpload, asynq.Queue(queues.TypeUploadVideoQueue)); err != nil {
		return fmt.Errorf("failed to enqueue image uploader task: %v", err)
	}

	return nil
}

func (*JobDownloadVideo) downloadVideo(ctx context.Context, filename string, outputPath string, urlVideo string) (string, error) {
	if err := utils.CreateFolder(outputPath); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	outputFilePath := fmt.Sprintf("%s/%s", outputPath, filename)
	if exists, err := utils.FileExists(outputFilePath); err != nil {
		return "", fmt.Errorf("error checking if file exists: %v", err)
	} else if exists {
		if err := utils.RemoveFile(outputFilePath); err != nil {
			return "", fmt.Errorf("failed to remove existing file: %v", err)
		}
	}

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/mp4",
		"-o", outputFilePath,
		urlVideo,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v, output: %s", err, output)
	}

	fmt.Printf("Command output: %s\n", output)

	//check if download was successful by checking if the file exists
	if exists, err := utils.FileExists(outputFilePath); err != nil || !exists {
		return "", fmt.Errorf("download failed, file does not exist: %s, error: %v", outputFilePath, err)
	}

	return outputFilePath, nil
}
