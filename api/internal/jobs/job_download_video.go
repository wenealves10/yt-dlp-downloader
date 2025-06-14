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

type JobDownloadVideo struct {
	client *asynq.Client
}

func NewJobDownloadVideo(client *asynq.Client) *JobDownloadVideo {
	return &JobDownloadVideo{
		client: client,
	}
}

func (p *JobDownloadVideo) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.DownloadVideoPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	filename := fmt.Sprintf("video_%s.mp4", uuid.New().String())
	outputPath := "./uploads/videos"

	outputFilePath, err := p.downloadVideo(ctx, filename, outputPath, payload.VideoURL)
	if err != nil {
		return err
	}

	//send image uploader task
	taskUpload, err := tasks.NewUploadVideoTask(outputFilePath, filename)
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
