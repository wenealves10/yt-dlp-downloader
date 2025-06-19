package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type UploadVideoPayload struct {
	DownloadID string `json:"download_id"`
	VideoPath  string `json:"video_path"`
	Filename   string `json:"filename"`
}

func NewUploadVideoTask(videoPath, downloadID, filename string) (*asynq.Task, error) {
	payload, err := json.Marshal(UploadVideoPayload{
		DownloadID: downloadID,
		VideoPath:  videoPath,
		Filename:   filename,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUploadVideo, payload), nil
}
