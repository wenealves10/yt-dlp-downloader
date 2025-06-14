package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

type JobDownloadMusic struct {
	client *asynq.Client
}

func NewJobDownloadMusic(client *asynq.Client) *JobDownloadMusic {
	return &JobDownloadMusic{
		client: client,
	}
}

func (p *JobDownloadMusic) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.DownloadMusicPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	filename := fmt.Sprintf("music_%s.mp3", uuid.New().String())
	outputPath := "./uploads/musics"

	outputFilePath, err := p.downloadMusic(ctx, filename, outputPath, payload.MusicURL)
	if err != nil {
		return err
	}

	//send uploader task
	taskUpload, err := tasks.NewUploadMusicTask(outputFilePath, filename)
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
