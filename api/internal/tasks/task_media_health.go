package tasks

import "github.com/hibiken/asynq"

// NewMediaHealthTask publica periodicamente o diagnóstico do provider do
// worker, para o painel administrativo poder lê-lo.
func NewMediaHealthTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeMediaHealth, nil), nil
}
