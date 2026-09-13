package jobs

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// diasDeRetencaoPadrao é usado quando a configuração não diz outra coisa.
//
// Trinta dias cobrem o ciclo útil dessas tabelas: uma investigação de uso
// indevido ou de webhook não entregue acontece em dias, não em meses. Guardar
// mais tempo custaria espaço e tornaria o painel mais lento para responder as
// perguntas que importam, que são sempre sobre o período recente.
const diasDeRetencaoPadrao = 30

// JobIntegrationPrune poda a auditoria das integrações.
//
// As duas tabelas que ele limpa crescem por REQUISIÇÃO e por NOTIFICAÇÃO, não
// por download — em uma integração ativa, elas passam o histórico de downloads
// em volume rapidamente. Sem poda, a tabela de requisições vira a maior do
// banco guardando dado que ninguém vai consultar.
type JobIntegrationPrune struct {
	store db.Store
	dias  int32
}

func NewJobIntegrationPrune(store db.Store, dias int) *JobIntegrationPrune {
	if dias <= 0 {
		dias = diasDeRetencaoPadrao
	}
	return &JobIntegrationPrune{store: store, dias: int32(dias)}
}

func (p *JobIntegrationPrune) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	requisicoes, err := p.store.PruneIntegrationRequests(ctx, p.dias)
	if err != nil {
		// Devolve o erro para o asynq tentar de novo: uma poda que falhou
		// silenciosamente só é descoberta quando o disco enche.
		return err
	}

	// As entregas que FALHARAM sobrevivem três vezes mais tempo, e a regra está
	// na própria consulta: é nelas que alguém vai procurar o motivo, muito
	// depois do dia em que aconteceram.
	entregas, err := p.store.PruneWebhookDeliveries(ctx, p.dias)
	if err != nil {
		return err
	}

	if requisicoes > 0 || entregas > 0 {
		log.Printf("integracoes: poda concluída requisicoes=%d entregas=%d retencao=%dd",
			requisicoes, entregas, p.dias)
	}
	return nil
}
