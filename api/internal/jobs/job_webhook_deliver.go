package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
)

// JobWebhookDeliver entrega uma notificação ao sistema integrado.
//
// A entrega roda no WORKER, e não na API, por uma razão só: ela depende de um
// servidor de terceiro responder. Um endpoint lento na mão de um cliente não
// pode ocupar a goroutine que atende as requisições HTTP de todos os outros —
// e é justamente o cliente com problema que demora mais.
//
// A política de reenvio é a do asynq (backoff exponencial entre tentativas).
// Reimplementá-la com uma coluna `next_attempt_at` e um job varredor seria
// refazer, com menos garantias, o que a fila já faz.
type JobWebhookDeliver struct {
	store      db.Store
	entregador *integrations.Entregador
	maxFalhas  int32
}

func NewJobWebhookDeliver(store db.Store, entregador *integrations.Entregador) *JobWebhookDeliver {
	return &JobWebhookDeliver{
		store:      store,
		entregador: entregador,
		maxFalhas:  integrations.FalhasParaDesligar,
	}
}

func (p *JobWebhookDeliver) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload integrations.PayloadEntrega
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// Payload ilegível não melhora com nova tentativa.
		return fmt.Errorf("payload de entrega inválido: %v: %w", err, asynq.SkipRetry)
	}

	entregaID, err := uuid.Parse(payload.DeliveryID)
	if err != nil {
		return fmt.Errorf("identificador de entrega inválido: %w", asynq.SkipRetry)
	}

	entrega, err := p.store.GetWebhookDelivery(ctx, entregaID)
	if err != nil {
		// A entrega pode ter sido podada pela retenção antes de a tarefa ser
		// processada. Insistir não a traria de volta.
		return fmt.Errorf("entrega %s não encontrada: %v: %w", entregaID, err, asynq.SkipRetry)
	}

	// Já entregue: a tarefa chegou duas vezes (reenfileiramento manual, ou um
	// reenvio do asynq após a gravação do sucesso). Repetir o POST mandaria o
	// mesmo evento de novo ao cliente sem necessidade.
	if entrega.Status == db.CoreWebhookDeliveryStatusDELIVERED {
		return nil
	}

	webhook, err := p.store.GetIntegrationWebhook(ctx, entrega.WebhookID)
	if err != nil {
		return fmt.Errorf("webhook %s não encontrado: %v: %w", entrega.WebhookID, err, asynq.SkipRetry)
	}

	// Webhook desligado no meio da fila: o administrador (ou o desligamento
	// automático) decidiu parar, e a fila ainda tem eventos antigos. Entregar
	// agora contrariaria a decisão que acabou de ser tomada.
	if !webhook.Active || webhook.DeletedAt.Valid {
		p.gravarResultado(ctx, entrega, db.CoreWebhookDeliveryStatusFAILED, integrations.Resultado{
			Erro: fmt.Errorf("webhook inativo no momento da entrega"),
		})
		return nil
	}

	tentativa := int(entrega.Attempts) + 1
	resultado := p.entregador.Entregar(ctx, webhook.Url, webhook.Secret,
		entrega.EventType, entrega.ID.String(), tentativa, entrega.Payload)

	if resultado.Sucesso() {
		p.gravarResultado(ctx, entrega, db.CoreWebhookDeliveryStatusDELIVERED, resultado)
		if err := p.store.MarkWebhookDelivered(ctx, db.MarkWebhookDeliveredParams{
			ID:             webhook.ID,
			LastStatusCode: pgtype.Int4{Int32: int32(resultado.StatusCode), Valid: true},
		}); err != nil {
			log.Printf("webhook: falha ao marcar entrega de %s: %v", webhook.ID, err)
		}
		log.Printf("webhook: entregue evento=%s webhook=%s status=%d em %dms tentativa=%d",
			entrega.EventType, webhook.ID, resultado.StatusCode,
			resultado.Duracao.Milliseconds(), tentativa)
		return nil
	}

	// A partir daqui é falha. Três desfechos possíveis, e a diferença entre
	// eles é o que separa um endpoint oscilando de um endpoint morto.
	definitivo := resultado.Definitivo() || !resultado.DeveRepetir()
	ultimaTentativa := tentativaFinal(ctx)

	log.Printf("webhook: falha evento=%s webhook=%s status=%d tentativa=%d definitivo=%t erro=%v",
		entrega.EventType, webhook.ID, resultado.StatusCode, tentativa, definitivo, resultado.Erro)

	if !definitivo && !ultimaTentativa {
		// Ainda há reenvio pela frente: a entrega permanece PENDING, para o
		// painel não mostrar como falhada algo que ainda vai ser tentado.
		p.gravarResultado(ctx, entrega, db.CoreWebhookDeliveryStatusPENDING, resultado)
		return p.erroParaAsynq(resultado)
	}

	p.gravarResultado(ctx, entrega, db.CoreWebhookDeliveryStatusFAILED, resultado)

	// O contador de falhas do webhook só sobe quando uma entrega é ABANDONADA,
	// e não a cada tentativa. Contando tentativas, uma única indisponibilidade
	// de meia hora somaria oito falhas e o desligamento automático puniria
	// quem apenas reiniciou o servidor.
	atualizado, err := p.store.MarkWebhookFailed(ctx, db.MarkWebhookFailedParams{
		ID:         webhook.ID,
		StatusCode: statusOuNulo(resultado),
		Error:      pgtype.Text{String: mensagemDeErro(resultado), Valid: true},
	})
	if err != nil {
		log.Printf("webhook: falha ao registrar falha de %s: %v", webhook.ID, err)
		return nil
	}

	if resultado.Definitivo() {
		p.desligar(ctx, webhook.ID,
			"o endpoint respondeu 410 Gone, pedindo para não receber mais notificações")
		return nil
	}

	if atualizado.ConsecutiveFailures >= p.maxFalhas {
		p.desligar(ctx, webhook.ID, fmt.Sprintf(
			"desativado automaticamente após %d entregas seguidas sem sucesso; último erro: %s",
			atualizado.ConsecutiveFailures, mensagemDeErro(resultado)))
	}

	// Não devolve erro: a entrega foi encerrada de propósito, e devolvê-lo
	// faria o asynq reenviar algo que acabou de ser abandonado.
	return nil
}

// gravarResultado registra a tentativa na linha da entrega.
func (p *JobWebhookDeliver) gravarResultado(
	ctx context.Context, entrega db.IntegrationWebhookDelivery,
	status db.CoreWebhookDeliveryStatus, resultado integrations.Resultado,
) {
	// Contexto próprio: o da tarefa pode estar cancelado justamente pelo
	// timeout que causou a falha, e perder o registro dela deixaria o painel
	// sem a explicação.
	gravacaoCtx, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelar()

	var erro pgtype.Text
	if mensagem := mensagemDeErro(resultado); mensagem != "" {
		erro = pgtype.Text{String: truncarErro(mensagem), Valid: true}
	}

	if err := p.store.UpdateWebhookDeliveryResult(gravacaoCtx, db.UpdateWebhookDeliveryResultParams{
		ID:         entrega.ID,
		Status:     status,
		DurationMs: int32(resultado.Duracao.Milliseconds()),
		StatusCode: statusOuNulo(resultado),
		Error:      erro,
	}); err != nil {
		log.Printf("webhook: falha ao gravar resultado da entrega %s: %v", entrega.ID, err)
	}
}

func (p *JobWebhookDeliver) desligar(ctx context.Context, webhookID uuid.UUID, motivo string) {
	gravacaoCtx, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelar()

	if err := p.store.DisableIntegrationWebhook(gravacaoCtx, db.DisableIntegrationWebhookParams{
		ID:             webhookID,
		DisabledReason: pgtype.Text{String: motivo, Valid: true},
	}); err != nil {
		log.Printf("webhook: falha ao desativar %s: %v", webhookID, err)
		return
	}
	log.Printf("webhook: desativado automaticamente %s: %s", webhookID, motivo)
}

// erroParaAsynq devolve o erro que faz o asynq reenviar.
func (p *JobWebhookDeliver) erroParaAsynq(resultado integrations.Resultado) error {
	if resultado.Erro != nil {
		return resultado.Erro
	}
	return fmt.Errorf("o endpoint respondeu %d", resultado.StatusCode)
}

// tentativaFinal diz se esta é a última tentativa que o asynq fará.
//
// Sem isto, a última falha deixaria a entrega como PENDING para sempre: o
// asynq desiste em silêncio e ninguém volta para fechar a linha.
func tentativaFinal(ctx context.Context) bool {
	tentativas, okT := asynq.GetRetryCount(ctx)
	maximo, okM := asynq.GetMaxRetry(ctx)
	if !okT || !okM {
		return false
	}
	return tentativas >= maximo
}

// mensagemDeErro descreve a falha em uma linha.
func mensagemDeErro(resultado integrations.Resultado) string {
	if resultado.Erro != nil {
		return resultado.Erro.Error()
	}
	if resultado.StatusCode >= 400 {
		if resultado.Corpo != "" {
			// O corpo da resposta do cliente é a parte mais útil do
			// diagnóstico: normalmente é ele que diz o que faltou no payload.
			return fmt.Sprintf("HTTP %d: %s", resultado.StatusCode, resultado.Corpo)
		}
		return fmt.Sprintf("HTTP %d", resultado.StatusCode)
	}
	return ""
}

func statusOuNulo(resultado integrations.Resultado) pgtype.Int4 {
	if resultado.StatusCode == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(resultado.StatusCode), Valid: true}
}

// truncarErro limita o que vai para o banco. O corpo vem de um servidor de
// terceiro; sem teto, um endpoint que responde uma página de erro inteira
// gravaria kilobytes por tentativa.
func truncarErro(mensagem string) string {
	const limite = 1000
	if len(mensagem) <= limite {
		return mensagem
	}
	return mensagem[:limite] + "…"
}
