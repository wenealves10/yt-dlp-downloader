package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type DownloadVideoPayload struct {
	DownloadID string `json:"download_id"`
}

func NewDownloadVideoTask(downloadID string) (*asynq.Task, error) {
	payload, err := json.Marshal(DownloadVideoPayload{
		DownloadID: downloadID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDownloadVideo, payload), nil
}
