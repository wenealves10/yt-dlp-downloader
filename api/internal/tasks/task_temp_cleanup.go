package tasks

import "github.com/hibiken/asynq"

// NewTempCleanupTask remove periodicamente os temporários órfãos. A limpeza na
// subida só resolve o que ficou de um restart; esta cobre o processo que roda
// por semanas sem reiniciar.
func NewTempCleanupTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeTempCleanup, nil), nil
}
