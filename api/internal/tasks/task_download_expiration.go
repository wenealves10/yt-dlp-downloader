package tasks

import (
	"github.com/hibiken/asynq"
)

func NewDownloadExpirationTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeDownloadExpiration, nil), nil
}
