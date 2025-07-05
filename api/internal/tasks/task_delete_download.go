package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type DeleteDownloadPayload struct {
	DownloadID string `json:"download_id"`
}

func NewDeleteDownloadTask(downloadID string) (*asynq.Task, error) {
	payload, err := json.Marshal(DeleteDownloadPayload{
		DownloadID: downloadID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDeleteDownload, payload), nil
}
