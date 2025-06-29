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
	"github.com/wenealves10/yt-dlp-downloader/internal/helpers"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

type JobDownloadMusic struct {
	client   *asynq.Client
	store    db.Store
	rdStream stream.EventPublisher
}

func NewJobDownloadMusic(client *asynq.Client, store db.Store, rdStream stream.EventPublisher) *JobDownloadMusic {
	return &JobDownloadMusic{
		client:   client,
		store:    store,
		rdStream: rdStream,
	}
}

func (p *JobDownloadMusic) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.DownloadMusicPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	filename := fmt.Sprintf("music_%s.mp3", uuid.New().String())
	outputPath := "./uploads/musics"

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

	outputFilePath, err := p.downloadMusic(ctx, filename, outputPath, downloadExists.OriginalUrl)
	if err != nil {
		return err
	}

	imagemBannerLocal := fmt.Sprintf("./uploads/musics/banners/%s_banner.jpg", uuid.New().String())
	err = helpers.DownloadImage(ctx, downloadExists.ThumbnailUrl.String, imagemBannerLocal)
	if err != nil {
		return fmt.Errorf("failed to download music thumbnail: %v", err)
	}

	//send uploader task
	taskUpload, err := tasks.NewUploadMusicTask(outputFilePath, payload.DownloadID, filename, imagemBannerLocal)
	if err != nil {
		return fmt.Errorf("failed to create upload music task: %v", err)
	}

	// Enqueue uploader task
	if _, err := p.client.Enqueue(taskUpload, asynq.Queue(queues.TypeUploadMusicQueue)); err != nil {
		return fmt.Errorf("failed to enqueue uploader task: %v", err)
	}

	return nil
}

func (*JobDownloadMusic) downloadMusic(ctx context.Context, filename string, outputPath string, urlMusic string) (string, error) {
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
		"-x",                    // Extrai o áudio
		"--audio-format", "mp3", // Converte para MP3
		"--audio-quality", "0", // Qualidade máxima
		"-o", outputFilePath, // Caminho de saída
		urlMusic, // URL do vídeo/música
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp command failed: %v, output: %s", err, output)
	}

	fmt.Printf("yt-dlp output:\n%s\n", output)

	//check if download was successful by checking if the file exists
	if exists, err := utils.FileExists(outputFilePath); err != nil || !exists {
		return "", fmt.Errorf("download failed, file does not exist: %s, error: %v", outputFilePath, err)
	}

	return outputFilePath, nil
}
