package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type DownloadVideoPayload struct {
	VideoURL string `json:"video_url"`
}

func NewDownloadVideoTask(videoURL string) (*asynq.Task, error) {
	payload, err := json.Marshal(DownloadVideoPayload{
		VideoURL: videoURL,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDownloadVideo, payload), nil
}
