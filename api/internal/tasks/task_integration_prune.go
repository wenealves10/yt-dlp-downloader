package tasks

import (
	"github.com/hibiken/asynq"
)

// NewIntegrationPruneTask cria a tarefa de poda da auditoria das integrações.
//
// Sem payload: a retenção é configuração do processo, não do agendamento. Se
// fosse embutida na tarefa, mudar a política exigiria esperar as tarefas
// antigas saírem da fila para o valor novo passar a valer.
func NewIntegrationPruneTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeIntegrationPrune, nil), nil
}
