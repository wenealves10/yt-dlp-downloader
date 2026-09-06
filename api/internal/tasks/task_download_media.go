package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// DownloadMediaPayload é o pedido do worker. Carrega só o identificador: tudo
// mais é lido do banco, que é a fonte da verdade e pode ter mudado entre o
// enfileiramento e a execução (um cancelamento, por exemplo).
type DownloadMediaPayload struct {
	DownloadID string `json:"download_id"`
}

func NewDownloadMediaTask(downloadID string) (*asynq.Task, error) {
	payload, err := json.Marshal(DownloadMediaPayload{DownloadID: downloadID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDownloadMedia, payload), nil
}
