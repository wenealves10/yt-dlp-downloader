package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
)

// A API que os OUTROS SISTEMAS consomem.
//
// Duas regras valem para tudo neste arquivo:
//
//  1. o objeto download devolvido é sempre o de
//     integrations.MontarDownloadPayload — o MESMO que vai no corpo do webhook.
//     Um cliente escreve um parser, não dois.
//  2. todo erro sai com `error` e `code`. A mensagem é para o humano que lê o
//     log; o código é para o `switch` do programa.

// baseURLPublica é o prefixo usado nos links devolvidos.
func (s *Server) baseURLPublica() string {
	return strings.TrimRight(s.config.PublicAPIURL, "/")
}

// payloadDownload monta a visão pública de um download.
func (s *Server) payloadDownload(download db.Download) integrations.DownloadPayload {
	return integrations.MontarDownloadPayload(download, s.baseURLPublica())
}

// ---------------------------------------------------------------------------
// Identidade e cota
// ---------------------------------------------------------------------------

// integrationMe devolve quem a chave representa e sob quais limites.
//
// É a primeira chamada de qualquer integração nova: confirma que a credencial
// funciona, informa o identificador público e mostra os tetos em vigor — sem
// que ninguém precise perguntar por e-mail quanto é a cota.
func (s *Server) integrationMe(ctx *gin.Context) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return
	}

	usados, _ := s.store.CountIntegrationDownloadsToday(ctx.Request.Context(), integracao.UserID)
	emAndamento, _ := s.store.CountIntegrationActiveDownloads(ctx.Request.Context(), integracao.UserID)

	ctx.JSON(http.StatusOK, gin.H{
		"integration": gin.H{
			"id":          integracao.ID.String(),
			"name":        integracao.Name,
			"description": integracao.Description.String,
			"active":      integracao.Active,
			"created_at":  integracao.CreatedAt,
		},
		"limits": gin.H{
			"daily_downloads":          integracao.DailyLimit,
			"max_concurrent_downloads": integracao.MaxConcurrentDownloads,
			"rate_limit_per_minute":    integracao.RateLimitPerMinute,
			"max_file_size_bytes":      integracao.MaxFileSizeBytes,
			// A lista de IPs é devolvida porque diagnosticar "por que recebi
			// ip_not_allowed" sem poder ver a lista configurada é impossível do
			// lado do cliente. Não é segredo: é a configuração dele.
			"allowed_ips": integracao.AllowedIps,
		},
		"usage": s.corpoCota(integracao, usados, emAndamento),
	})
}

// integrationQuota é a rota enxuta da cota, para o cliente consultar antes de
// disparar um lote.
func (s *Server) integrationQuota(ctx *gin.Context) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return
	}

	usados, err := s.store.CountIntegrationDownloadsToday(ctx.Request.Context(), integracao.UserID)
	if err != nil {
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao consultar a cota")
		return
	}
	emAndamento, err := s.store.CountIntegrationActiveDownloads(ctx.Request.Context(), integracao.UserID)
	if err != nil {
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao consultar a cota")
		return
	}

	ctx.JSON(http.StatusOK, s.corpoCota(integracao, usados, emAndamento))
}

// corpoCota monta o relato de consumo.
//
// `resets_at` existe para o cliente poder AGENDAR em vez de tentar: sem ele, a
// reação natural a uma cota esgotada é repetir a chamada em laço até funcionar.
func (s *Server) corpoCota(integracao db.Integration, usados, emAndamento int64) gin.H {
	restante := int64(integracao.DailyLimit) - usados
	if restante < 0 {
		restante = 0
	}

	return gin.H{
		"daily_limit":        integracao.DailyLimit,
		"daily_used":         usados,
		"daily_remaining":    restante,
		"unlimited":          integracao.DailyLimit == 0,
		"downloads_active":   emAndamento,
		"concurrency_limit":  integracao.MaxConcurrentDownloads,
		"resets_at":          proximoResetCota().Format(time.RFC3339),
		"resets_at_timezone": "America/Sao_Paulo",
	}
}

// ---------------------------------------------------------------------------
// Downloads
// ---------------------------------------------------------------------------

type integrationDownloadRequest struct {
	URL string `json:"url" binding:"required"`
	// FormatID vem de POST /media/resolve. Vazio usa o melhor formato do tipo.
	FormatID string `json:"format_id"`
	// Kind escolhe entre vídeo e áudio (que é o que decide a conversão p/ MP3).
	Kind string `json:"kind" binding:"omitempty,oneof=video audio"`
}

// integrationCreateDownload cria o download e responde 202.
//
// 202 e não 201: o recurso foi ACEITO para processamento, e o arquivo não
// existe ainda. Responder 201 sugeriria que o download está pronto para ser
// buscado, e o cliente que confia no código HTTP iria buscá-lo na hora.
func (s *Server) integrationCreateDownload(ctx *gin.Context) {
	var req integrationDownloadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		erroIntegracao(ctx, http.StatusBadRequest, CodeInvalidRequest,
			"informe ao menos o campo url")
		return
	}

	download, ok := s.prepararDownloadMedia(ctx, criarDownloadMediaRequest{
		URL:      req.URL,
		FormatID: req.FormatID,
		Kind:     req.Kind,
	})
	if !ok {
		return
	}

	integracao, _ := currentIntegration(ctx)
	log.Printf("integracoes: download criado id=%s integracao=%s plataforma=%s",
		download.ID, integracao.ID, download.Platform)

	ctx.JSON(http.StatusAccepted, gin.H{"download": s.payloadDownload(download)})
}

// integrationCancelDownload cancela e devolve o download já atualizado.
//
// O cancelamento em si é o mesmo da tela (a chave no Redis antes do UPDATE, que
// é o que alcança um processo já em andamento); só o corpo da resposta muda,
// para manter a promessa de que toda rota desta API devolve o mesmo objeto.
func (s *Server) integrationCancelDownload(ctx *gin.Context) {
	// A conferência de dono é do cancelamento compartilhado: ele filtra por
	// user_id, que aqui é a conta de serviço desta integração.
	download, ok := s.cancelarDownload(ctx)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"download": s.payloadDownload(download)})
}

// integrationListDownloads lista os downloads da integração.
func (s *Server) integrationListDownloads(ctx *gin.Context) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return
	}

	limit, offset, page, perPage := paginacao(ctx)
	status, valido := filtroStatusDownload(ctx)
	if !valido {
		erroIntegracao(ctx, http.StatusBadRequest, CodeInvalidRequest,
			"status inválido; use PENDING, PROCESSING, COMPLETED, FAILED, CANCELED, EXPIRED ou RETRYING")
		return
	}

	filtros := db.ListIntegrationDownloadsParams{
		UserID:   integracao.UserID,
		Limit:    int32(limit),
		Offset:   int32(offset),
		Status:   status,
		Platform: filtroTexto(ctx, "platform"),
		Search:   filtroTexto(ctx, "search"),
	}

	total, err := s.store.CountIntegrationDownloads(ctx.Request.Context(), db.CountIntegrationDownloadsParams{
		UserID:   filtros.UserID,
		Status:   filtros.Status,
		Platform: filtros.Platform,
		Search:   filtros.Search,
	})
	if err != nil {
		log.Printf("integracoes: falha ao contar downloads de %s: %v", integracao.ID, err)
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao listar downloads")
		return
	}

	linhas, err := s.store.ListIntegrationDownloads(ctx.Request.Context(), filtros)
	if err != nil {
		log.Printf("integracoes: falha ao listar downloads de %s: %v", integracao.ID, err)
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao listar downloads")
		return
	}

	itens := make([]integrations.DownloadPayload, 0, len(linhas))
	for _, linha := range linhas {
		itens = append(itens, s.payloadDownload(linha))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"downloads": itens,
		"pagination": gin.H{
			"page":      page,
			"per_page":  perPage,
			"total":     total,
			"next_page": int64(offset+len(linhas)) < total,
			"prev_page": page > 1,
		},
	})
}

// integrationGetDownload devolve um download específico.
//
// É a rota de consulta de estado, e é a alternativa oficial ao webhook: quem
// não pode expor um endpoint público consulta esta rota. A documentação pede
// espaçamento razoável — o limitador por minuto é o que garante isso.
func (s *Server) integrationGetDownload(ctx *gin.Context) {
	download, ok := s.downloadDaIntegracao(ctx)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"download": s.payloadDownload(download)})
}

// downloadDaIntegracao carrega o download conferindo que ele é DESTA
// integração.
//
// A conferência de dono fica em um lugar só, e responde 404 quando falha: com a
// checagem espalhada, basta uma rota nova esquecer dela para um sistema
// conseguir ler o download de outro.
func (s *Server) downloadDaIntegracao(ctx *gin.Context) (db.Download, bool) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return db.Download{}, false
	}

	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		erroIntegracao(ctx, http.StatusBadRequest, CodeInvalidRequest, "identificador de download inválido")
		return db.Download{}, false
	}

	download, err := s.store.GetDownloadByID(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			erroIntegracao(ctx, http.StatusNotFound, CodeNotFound, "download não encontrado")
			return db.Download{}, false
		}
		log.Printf("integracoes: falha ao carregar download %s: %v", id, err)
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao carregar o download")
		return db.Download{}, false
	}

	if download.UserID != integracao.UserID {
		erroIntegracao(ctx, http.StatusNotFound, CodeNotFound, "download não encontrado")
		return db.Download{}, false
	}

	return download, true
}

// ---------------------------------------------------------------------------
// Webhooks (leitura)
// ---------------------------------------------------------------------------

// integrationListWebhooks mostra a configuração de notificação.
//
// O SEGREDO DE ASSINATURA NÃO SAI AQUI, e isso é deliberado: ele é entregue
// pelo painel a uma pessoa. Devolvê-lo por uma rota autenticada pela chave de
// API faria de um vazamento de chave também um vazamento do segredo — e com o
// segredo em mãos um atacante forjaria eventos assinados para o endpoint do
// cliente, que os aceitaria como nossos.
func (s *Server) integrationListWebhooks(ctx *gin.Context) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return
	}

	webhooks, err := s.store.ListIntegrationWebhooks(ctx.Request.Context(), integracao.ID)
	if err != nil {
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao listar webhooks")
		return
	}

	itens := make([]gin.H, 0, len(webhooks))
	for _, webhook := range webhooks {
		itens = append(itens, gin.H{
			"id":                   webhook.ID.String(),
			"url":                  webhook.Url,
			"events":               eventosDoWebhook(webhook),
			"include_progress":     webhook.IncludeProgress,
			"active":               webhook.Active,
			"last_delivery_at":     instanteOuNil(webhook.LastDeliveryAt),
			"last_status_code":     webhook.LastStatusCode.Int32,
			"consecutive_failures": webhook.ConsecutiveFailures,
			"disabled_reason":      webhook.DisabledReason.String,
			"created_at":           webhook.CreatedAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"webhooks": itens})
}

// integrationTestWebhook dispara um evento de teste.
//
// O cliente pode chamar esta rota, e é seguro: o destino é a URL que o
// administrador cadastrou, não uma informada na requisição. É o que permite ao
// time do cliente validar a verificação de assinatura sem esperar um download
// real acontecer.
func (s *Server) integrationTestWebhook(ctx *gin.Context) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return
	}

	webhook, ok := s.webhookDe(ctx, integracao.ID)
	if !ok {
		return
	}

	if err := s.dispatcher.Enfileirar(ctx.Request.Context(), webhook, integracao.ID,
		integrations.EventPing, integrations.DownloadPayload{}); err != nil {
		log.Printf("integracoes: falha ao enfileirar teste do webhook %s: %v", webhook.ID, err)
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError,
			"não foi possível enfileirar o evento de teste")
		return
	}

	// 202: o POST no endpoint do cliente acontece em segundo plano. Responder
	// 200 aqui afirmaria que a entrega deu certo, o que ainda não se sabe — o
	// resultado aparece na listagem de entregas.
	ctx.JSON(http.StatusAccepted, gin.H{
		"message": "evento de teste enfileirado; consulte GET /v1/integration/deliveries para o resultado",
	})
}

// integrationListDeliveries mostra o histórico de notificações.
//
// Existe para encerrar a discussão mais comum de qualquer integração por
// webhook — "vocês enviaram?" / "não recebi" — com um registro que as duas
// partes podem consultar: o que foi enviado, quando, e o que o endpoint
// respondeu.
func (s *Server) integrationListDeliveries(ctx *gin.Context) {
	integracao, ok := currentIntegration(ctx)
	if !ok {
		erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials, "requisição não autenticada")
		return
	}

	limit, offset, page, perPage := paginacao(ctx)

	status, valido := filtroStatusEntrega(ctx)
	if !valido {
		erroIntegracao(ctx, http.StatusBadRequest, CodeInvalidRequest,
			"status inválido; use PENDING, DELIVERED ou FAILED")
		return
	}

	var downloadID pgtype.UUID
	if bruto := strings.TrimSpace(ctx.Query("download_id")); bruto != "" {
		id, err := uuid.Parse(bruto)
		if err != nil {
			erroIntegracao(ctx, http.StatusBadRequest, CodeInvalidRequest, "download_id inválido")
			return
		}
		downloadID = pgtype.UUID{Bytes: id, Valid: true}
	}

	total, err := s.store.CountWebhookDeliveries(ctx.Request.Context(), db.CountWebhookDeliveriesParams{
		IntegrationID: integracao.ID,
		Status:        status,
		EventType:     filtroTexto(ctx, "event_type"),
		DownloadID:    downloadID,
	})
	if err != nil {
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao listar entregas")
		return
	}

	linhas, err := s.store.ListWebhookDeliveries(ctx.Request.Context(), db.ListWebhookDeliveriesParams{
		IntegrationID: integracao.ID,
		Limit:         int32(limit),
		Offset:        int32(offset),
		Status:        status,
		EventType:     filtroTexto(ctx, "event_type"),
		DownloadID:    downloadID,
	})
	if err != nil {
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError, "falha ao listar entregas")
		return
	}

	itens := make([]gin.H, 0, len(linhas))
	for _, linha := range linhas {
		itens = append(itens, corpoEntrega(linha))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"deliveries": itens,
		"pagination": gin.H{
			"page":      page,
			"per_page":  perPage,
			"total":     total,
			"next_page": int64(offset+len(linhas)) < total,
			"prev_page": page > 1,
		},
	})
}

// webhookDe carrega um webhook conferindo que ele pertence à integração.
func (s *Server) webhookDe(ctx *gin.Context, integrationID uuid.UUID) (db.IntegrationWebhook, bool) {
	id, err := uuid.Parse(ctx.Param("webhookId"))
	if err != nil {
		erroIntegracao(ctx, http.StatusBadRequest, CodeInvalidRequest, "identificador de webhook inválido")
		return db.IntegrationWebhook{}, false
	}

	webhook, err := s.store.GetIntegrationWebhook(ctx.Request.Context(), id)
	if err != nil || webhook.IntegrationID != integrationID {
		erroIntegracao(ctx, http.StatusNotFound, CodeNotFound, "webhook não encontrado")
		return db.IntegrationWebhook{}, false
	}
	return webhook, true
}

// ---------------------------------------------------------------------------
// Auxiliares de apresentação
// ---------------------------------------------------------------------------

// eventosDoWebhook devolve a lista EFETIVA de eventos.
//
// Guardar o array vazio e exibi-lo como vazio faria a tela e a API dizerem "não
// assina nada", quando na verdade o vazio significa "todos os de ciclo de
// vida". Resolver isso aqui evita que cada leitor repita a regra.
func eventosDoWebhook(webhook db.IntegrationWebhook) []string {
	if len(webhook.Events) > 0 {
		return webhook.Events
	}
	efetivos := append([]string{}, integrations.EventosDeCicloDeVida...)
	if webhook.IncludeProgress {
		efetivos = append(efetivos, integrations.EventDownloadProgress)
	}
	return efetivos
}

// corpoEntrega monta a linha de uma entrega para as duas telas que a mostram
// (a do painel e a da API).
func corpoEntrega(linha db.ListWebhookDeliveriesRow) gin.H {
	corpo := gin.H{
		"id":               linha.ID.String(),
		"webhook_id":       linha.WebhookID.String(),
		"webhook_url":      linha.WebhookUrl,
		"event_type":       linha.EventType,
		"status":           string(linha.Status),
		"attempts":         linha.Attempts,
		"duration_ms":      linha.DurationMs,
		"last_status_code": linha.LastStatusCode.Int32,
		"last_error":       linha.LastError.String,
		"created_at":       linha.CreatedAt,
		"delivered_at":     instanteOuNil(linha.DeliveredAt),
	}
	if linha.DownloadID.Valid {
		corpo["download_id"] = uuid.UUID(linha.DownloadID.Bytes).String()
	}
	// O payload vai como JSON já pronto, e não como string: reprocessá-lo
	// obrigaria o leitor a fazer um segundo parse do que já é JSON válido.
	//
	// A guarda não é paranoia inútil: json.RawMessage vazio faz o encoder
	// falhar, e uma linha estranha derrubaria a LISTAGEM INTEIRA com 500 em vez
	// de mostrar as outras entregas.
	if len(linha.Payload) > 0 {
		corpo["payload"] = json.RawMessage(linha.Payload)
	}
	return corpo
}

// filtroStatusDownload valida o status pedido na query.
//
// Um status desconhecido é ERRO, e não filtro ignorado: silenciosamente
// devolver a lista completa para quem pediu "?status=completed" (minúsculo)
// faria o cliente concluir que tudo está concluído.
func filtroStatusDownload(ctx *gin.Context) (db.NullCoreDownloadStatus, bool) {
	bruto := strings.TrimSpace(ctx.Query("status"))
	if bruto == "" {
		return db.NullCoreDownloadStatus{}, true
	}

	switch db.CoreDownloadStatus(bruto) {
	case db.CoreDownloadStatusPENDING, db.CoreDownloadStatusPROCESSING,
		db.CoreDownloadStatusCOMPLETED, db.CoreDownloadStatusFAILED,
		db.CoreDownloadStatusCANCELED, db.CoreDownloadStatusEXPIRED,
		db.CoreDownloadStatusRETRYING:
		return db.NullCoreDownloadStatus{
			CoreDownloadStatus: db.CoreDownloadStatus(bruto), Valid: true,
		}, true
	}
	return db.NullCoreDownloadStatus{}, false
}

// filtroStatusEntrega valida o status de entrega pedido na query.
func filtroStatusEntrega(ctx *gin.Context) (db.NullCoreWebhookDeliveryStatus, bool) {
	bruto := strings.TrimSpace(ctx.Query("status"))
	if bruto == "" {
		return db.NullCoreWebhookDeliveryStatus{}, true
	}

	switch db.CoreWebhookDeliveryStatus(bruto) {
	case db.CoreWebhookDeliveryStatusPENDING,
		db.CoreWebhookDeliveryStatusDELIVERED,
		db.CoreWebhookDeliveryStatusFAILED:
		return db.NullCoreWebhookDeliveryStatus{
			CoreWebhookDeliveryStatus: db.CoreWebhookDeliveryStatus(bruto), Valid: true,
		}, true
	}
	return db.NullCoreWebhookDeliveryStatus{}, false
}

// instanteOuNil devolve nil em vez de uma data zerada, para o JSON omitir o
// campo em vez de afirmar "0001-01-01".
func instanteOuNil(valor pgtype.Timestamptz) *time.Time {
	if !valor.Valid {
		return nil
	}
	momento := valor.Time
	return &momento
}
