package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type DownloadMusicPayload struct {
	DownloadID string `json:"download_id"`
}

func NewDownloadMusicTask(downloadID string) (*asynq.Task, error) {
	payload, err := json.Marshal(DownloadMusicPayload{
		DownloadID: downloadID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDownloadMusic, payload), nil
}
