package tasks

import (
	"github.com/hibiken/asynq"
)

// NewYoutubeHealthCheckTask verifica periodicamente se as sessões das contas do
// YouTube continuam autenticadas.
func NewYoutubeHealthCheckTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeYoutubeHealthCheck, nil), nil
}
