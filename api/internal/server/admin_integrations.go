package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

// O painel de integrações, exclusivo do super admin.
//
// Quem cria integração é SEMPRE uma pessoa autenticada no painel: não existe
// rota de auto-cadastro. É a mesma razão pela qual a criação de conta de
// usuário por API não existe aqui — um sistema que pudesse criar o próprio
// acesso tornaria a cota e a lista de IPs decorativas.

// janelaPadraoHoras é o período das métricas quando a tela não pede outro.
const janelaPadraoHoras = 24

// ---------------------------------------------------------------------------
// Integrações
// ---------------------------------------------------------------------------

type adminIntegrationResponse struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Description            string     `json:"description,omitempty"`
	CallbackBaseURL        string     `json:"callback_base_url,omitempty"`
	DailyLimit             int32      `json:"daily_limit"`
	MaxFileSizeBytes       int64      `json:"max_file_size_bytes"`
	RateLimitPerMinute     int32      `json:"rate_limit_per_minute"`
	MaxConcurrentDownloads int32      `json:"max_concurrent_downloads"`
	AllowedIPs             []string   `json:"allowed_ips"`
	Active                 bool       `json:"active"`
	ServiceEmail           string     `json:"service_email,omitempty"`
	CreatedAt              *time.Time `json:"created_at"`
	UpdatedAt              *time.Time `json:"updated_at"`

	// Números da listagem. Ficam no mesmo objeto porque a tabela mostra tudo
	// junto, e uma segunda chamada por linha transformaria a abertura da tela
	// em uma consulta por integração.
	DownloadsTotal  int64      `json:"downloads_total"`
	DownloadsToday  int64      `json:"downloads_today"`
	DownloadsActive int64      `json:"downloads_active"`
	StorageBytes    int64      `json:"storage_bytes"`
	ActiveKeys      int64      `json:"active_keys"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
}

func integracaoDeModelo(integracao db.Integration) adminIntegrationResponse {
	return adminIntegrationResponse{
		ID:                     integracao.ID.String(),
		Name:                   integracao.Name,
		Description:            integracao.Description.String,
		CallbackBaseURL:        integracao.CallbackBaseUrl.String,
		DailyLimit:             integracao.DailyLimit,
		MaxFileSizeBytes:       integracao.MaxFileSizeBytes,
		RateLimitPerMinute:     integracao.RateLimitPerMinute,
		MaxConcurrentDownloads: integracao.MaxConcurrentDownloads,
		AllowedIPs:             naoNulo(integracao.AllowedIps),
		Active:                 integracao.Active,
		CreatedAt:              integracao.CreatedAt,
		UpdatedAt:              integracao.UpdatedAt,
	}
}

// naoNulo garante array em vez de null no JSON. `null` obrigaria cada tela a
// tratar dois casos ("sem lista" e "lista vazia") que significam o mesmo.
func naoNulo(valores []string) []string {
	if valores == nil {
		return []string{}
	}
	return valores
}

func (s *Server) listIntegrations(ctx *gin.Context) {
	limit, offset, page, perPage := paginacao(ctx)
	busca, ativo := filtroTexto(ctx, "search"), filtroAtivo(ctx)

	total, err := s.store.CountIntegrations(ctx.Request.Context(), db.CountIntegrationsParams{
		Search: busca, Active: ativo,
	})
	if err != nil {
		log.Printf("admin: falha ao contar integrações: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar integrações")))
		return
	}

	linhas, err := s.store.ListIntegrations(ctx.Request.Context(), db.ListIntegrationsParams{
		Limit: int32(limit), Offset: int32(offset), Search: busca, Active: ativo,
	})
	if err != nil {
		log.Printf("admin: falha ao listar integrações: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar integrações")))
		return
	}

	itens := make([]adminIntegrationResponse, 0, len(linhas))
	for _, linha := range linhas {
		item := integracaoDeModelo(db.Integration{
			ID: linha.ID, UserID: linha.UserID, Name: linha.Name,
			Description: linha.Description, CallbackBaseUrl: linha.CallbackBaseUrl,
			DailyLimit: linha.DailyLimit, MaxFileSizeBytes: linha.MaxFileSizeBytes,
			RateLimitPerMinute:     linha.RateLimitPerMinute,
			MaxConcurrentDownloads: linha.MaxConcurrentDownloads,
			AllowedIps:             linha.AllowedIps, Active: linha.Active,
			CreatedAt: linha.CreatedAt, UpdatedAt: linha.UpdatedAt,
		})
		item.ServiceEmail = linha.ServiceEmail
		item.DownloadsTotal = linha.DownloadsTotal
		item.DownloadsToday = linha.DownloadsToday
		item.DownloadsActive = linha.DownloadsActive
		item.StorageBytes = linha.StorageBytes
		item.ActiveKeys = linha.ActiveKeys
		item.LastUsedAt = linha.LastUsedAt
		itens = append(itens, item)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"integrations": itens,
		"total":        total,
		"page":         page,
		"per_page":     perPage,
		"next_page":    int64(offset+len(linhas)) < total,
		"prev_page":    page > 1,
	})
}

type criarIntegracaoRequest struct {
	Name                   string   `json:"name" binding:"required,min=2,max=120"`
	Description            string   `json:"description" binding:"omitempty,max=500"`
	CallbackBaseURL        string   `json:"callback_base_url" binding:"omitempty,url,max=500"`
	DailyLimit             *int32   `json:"daily_limit" binding:"omitempty,min=0,max=100000"`
	MaxFileSizeBytes       *int64   `json:"max_file_size_bytes" binding:"omitempty,min=0"`
	RateLimitPerMinute     *int32   `json:"rate_limit_per_minute" binding:"omitempty,min=1,max=10000"`
	MaxConcurrentDownloads *int32   `json:"max_concurrent_downloads" binding:"omitempty,min=0,max=1000"`
	AllowedIPs             []string `json:"allowed_ips"`
}

// createIntegration cria a integração, a conta de serviço e a primeira chave,
// e devolve a chave em claro UMA ÚNICA VEZ.
//
// Os três nascem juntos porque separá-los deixaria estados sem sentido pelo
// caminho: uma integração sem conta não pode ter download, e uma sem chave não
// pode ser usada — e alguém teria de lembrar de completar o cadastro depois.
func (s *Server) createIntegration(ctx *gin.Context) {
	var req criarIntegracaoRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ips, err := integrations.NormalizarIPsPermitidos(req.AllowedIPs)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	admin, _ := currentUser(ctx)
	integrationID := utils.GenerateUUID()

	// A conta de serviço precisa de um e-mail (a coluna é única e obrigatória),
	// mas ela nunca recebe mensagem nem faz login. O domínio .invalid é
	// reservado pela RFC 2606 exatamente para isto: garante que nada aqui
	// possa ser confundido com um endereço real de alguém.
	emailServico := fmt.Sprintf("integracao-%s@sistemas.invalid", integrationID.String())

	// Senha impossível de usar. Não é uma senha aleatória que ninguém conhece:
	// é um valor que NÃO É um hash bcrypt válido, então a comparação falha
	// sempre, para qualquer entrada. Uma conta de serviço não deve ter um
	// caminho de login, nem improvável.
	senhaInutilizavel := "conta-de-servico-sem-senha:" + utils.GenerateUUIDString()

	contaServico, err := s.store.CreateServiceUser(ctx.Request.Context(), db.CreateServiceUserParams{
		ID:             utils.GenerateUUID(),
		FullName:       req.Name,
		Email:          emailServico,
		HashedPassword: senhaInutilizavel,
	})
	if err != nil {
		log.Printf("admin: falha ao criar conta de serviço: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New(db.HandleDBError(err))))
		return
	}

	integracao, err := s.store.CreateIntegration(ctx.Request.Context(), db.CreateIntegrationParams{
		ID:                     integrationID,
		UserID:                 contaServico.ID,
		Name:                   strings.TrimSpace(req.Name),
		Description:            textoOuNulo(req.Description),
		CallbackBaseUrl:        textoOuNulo(req.CallbackBaseURL),
		DailyLimit:             valorOuPadrao(req.DailyLimit, 50),
		MaxFileSizeBytes:       valorOuPadrao(req.MaxFileSizeBytes, 0),
		RateLimitPerMinute:     valorOuPadrao(req.RateLimitPerMinute, 60),
		MaxConcurrentDownloads: valorOuPadrao(req.MaxConcurrentDownloads, 3),
		AllowedIps:             ips,
		CreatedBy:              pgtype.UUID{Bytes: admin.ID, Valid: true},
	})
	if err != nil {
		log.Printf("admin: falha ao criar integração: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New(db.HandleDBError(err))))
		return
	}

	chave, valorEmClaro, err := s.emitirChave(ctx, integracao.ID, "chave inicial", nil)
	if err != nil {
		// A integração existe e está sem chave. Não é um estado quebrado: o
		// painel oferece "gerar nova chave", e desfazer a criação inteira por
		// causa disto perderia o cadastro que o administrador acabou de fazer.
		log.Printf("admin: integração %s criada sem chave: %v", integracao.ID, err)
		ctx.JSON(http.StatusCreated, gin.H{
			"integration": integracaoDeModelo(integracao),
			"warning":     "a integração foi criada, mas a chave inicial falhou; gere uma nova chave",
		})
		return
	}

	log.Printf("admin: integração criada id=%s nome=%q por=%s", integracao.ID, integracao.Name, admin.ID)

	resposta := integracaoDeModelo(integracao)
	resposta.ServiceEmail = contaServico.Email

	ctx.JSON(http.StatusCreated, gin.H{
		"integration": resposta,
		"api_key":     chaveDeModelo(chave),
		// O único momento da vida do sistema em que este valor existe fora da
		// memória do processo. O banco só tem o hash.
		"api_key_plaintext": valorEmClaro,
	})
}

type atualizarIntegracaoRequest struct {
	Name                   *string   `json:"name" binding:"omitempty,min=2,max=120"`
	Description            *string   `json:"description" binding:"omitempty,max=500"`
	CallbackBaseURL        *string   `json:"callback_base_url" binding:"omitempty,url,max=500"`
	DailyLimit             *int32    `json:"daily_limit" binding:"omitempty,min=0,max=100000"`
	MaxFileSizeBytes       *int64    `json:"max_file_size_bytes" binding:"omitempty,min=0"`
	RateLimitPerMinute     *int32    `json:"rate_limit_per_minute" binding:"omitempty,min=1,max=10000"`
	MaxConcurrentDownloads *int32    `json:"max_concurrent_downloads" binding:"omitempty,min=0,max=1000"`
	AllowedIPs             *[]string `json:"allowed_ips"`
	Active                 *bool     `json:"active"`
}

func (s *Server) updateIntegration(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	var req atualizarIntegracaoRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// O ponteiro para slice distingue "não mandei a lista" de "mandei vazia",
	// e essa diferença importa: a lista vazia é uma escolha (aceitar qualquer
	// IP), não a ausência de escolha.
	var ips []string
	if req.AllowedIPs != nil {
		normalizados, err := integrations.NormalizarIPsPermitidos(*req.AllowedIPs)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		// O array vazio precisa chegar à consulta como array, e não como nil:
		// nil é o sentinela de "não altere".
		if normalizados == nil {
			normalizados = []string{}
		}
		ips = normalizados
	}

	atualizada, err := s.store.UpdateIntegration(ctx.Request.Context(), db.UpdateIntegrationParams{
		ID:                     integracao.ID,
		Name:                   db.ToPgText(req.Name),
		Description:            db.ToPgText(req.Description),
		CallbackBaseUrl:        db.ToPgText(req.CallbackBaseURL),
		DailyLimit:             db.ToPgInt4(req.DailyLimit),
		MaxFileSizeBytes:       int8OuNulo(req.MaxFileSizeBytes),
		RateLimitPerMinute:     db.ToPgInt4(req.RateLimitPerMinute),
		MaxConcurrentDownloads: db.ToPgInt4(req.MaxConcurrentDownloads),
		AllowedIps:             ips,
		Active:                 db.ToPgBool(req.Active),
	})
	if err != nil {
		log.Printf("admin: falha ao atualizar integração %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao atualizar integração")))
		return
	}

	// O despachante mantém a integração em memória por até um minuto. Sem esta
	// invalidação, desativar uma integração no painel continuaria entregando
	// webhooks dela por esse tempo.
	s.dispatcher.InvalidarCache(atualizada.ID, atualizada.UserID)

	admin, _ := currentUser(ctx)
	log.Printf("admin: integração atualizada id=%s por=%s", atualizada.ID, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"integration": integracaoDeModelo(atualizada)})
}

// deleteIntegration remove logicamente e revoga todas as chaves.
//
// Revogar junto é o ponto: sem isso, a integração sairia da tela e as chaves
// dela continuariam existindo no banco. Elas não autenticariam (a checagem de
// deleted_at barra), mas ficariam como credenciais válidas esquecidas — e
// bastaria um "restaurar" futuro para reativá-las sem ninguém decidir isso.
func (s *Server) deleteIntegration(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	if err := s.store.RevokeOtherIntegrationAPIKeys(ctx.Request.Context(), db.RevokeOtherIntegrationAPIKeysParams{
		IntegrationID: integracao.ID,
		// uuid.Nil não existe como chave, então "todas menos esta" é "todas".
		KeepID: uuid.Nil,
		Reason: pgtype.Text{String: "integração removida", Valid: true},
	}); err != nil {
		log.Printf("admin: falha ao revogar chaves de %s: %v", integracao.ID, err)
	}

	if err := s.store.SoftDeleteIntegration(ctx.Request.Context(), integracao.ID); err != nil {
		log.Printf("admin: falha ao remover integração %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao remover integração")))
		return
	}

	s.dispatcher.InvalidarCache(integracao.ID, integracao.UserID)

	admin, _ := currentUser(ctx)
	log.Printf("admin: integração removida id=%s por=%s", integracao.ID, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"message": "integração removida"})
}

// getIntegration devolve a integração com os números da visão geral.
func (s *Server) getIntegration(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	janela := janelaHoras(ctx)

	resumo, err := s.store.IntegrationDownloadSummary(ctx.Request.Context(), integracao.UserID)
	if err != nil {
		log.Printf("admin: falha no resumo da integração %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar integração")))
		return
	}

	requisicoes, err := s.store.IntegrationRequestStats(ctx.Request.Context(), db.IntegrationRequestStatsParams{
		IntegrationID: integracao.ID, WindowHours: janela,
	})
	if err != nil {
		log.Printf("admin: falha nas estatísticas de %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar integração")))
		return
	}

	entregas, err := s.store.WebhookDeliveryStats(ctx.Request.Context(), db.WebhookDeliveryStatsParams{
		IntegrationID: integracao.ID, WindowHours: janela,
	})
	if err != nil {
		log.Printf("admin: falha nas entregas de %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar integração")))
		return
	}

	usados, _ := s.store.CountIntegrationDownloadsToday(ctx.Request.Context(), integracao.UserID)

	ctx.JSON(http.StatusOK, gin.H{
		"integration": integracaoDeModelo(integracao),
		"downloads": gin.H{
			"total":             resumo.Total,
			"today":             resumo.Today,
			"active":            resumo.Active,
			"completed":         resumo.Completed,
			"failed":            resumo.Failed,
			"canceled":          resumo.Canceled,
			"storage_bytes":     resumo.StorageBytes,
			"transferred_bytes": resumo.TransferredBytes,
		},
		"quota": gin.H{
			"daily_limit":     integracao.DailyLimit,
			"daily_used":      usados,
			"daily_remaining": restanteNaoNegativo(int64(integracao.DailyLimit) - usados),
			"resets_at":       proximoResetCota().Format(time.RFC3339),
		},
		"requests": gin.H{
			"window_hours":    janela,
			"total":           requisicoes.Total,
			"ok":              requisicoes.Ok,
			"client_errors":   requisicoes.ClientErrors,
			"server_errors":   requisicoes.ServerErrors,
			"throttled":       requisicoes.Throttled,
			"unique_ips":      requisicoes.UniqueIps,
			"avg_duration_ms": requisicoes.AvgDurationMs,
			"max_duration_ms": requisicoes.MaxDurationMs,
		},
		"deliveries": gin.H{
			"window_hours":    janela,
			"total":           entregas.Total,
			"delivered":       entregas.Delivered,
			"failed":          entregas.Failed,
			"pending":         entregas.Pending,
			"avg_duration_ms": entregas.AvgDurationMs,
		},
	})
}

// ---------------------------------------------------------------------------
// Chaves
// ---------------------------------------------------------------------------

type chaveResponse struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Prefix     string     `json:"prefix"`
	Masked     string     `json:"masked"`
	Revoked    bool       `json:"revoked"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	Reason     string     `json:"revoked_reason,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP string     `json:"last_used_ip,omitempty"`
	CreatedAt  *time.Time `json:"created_at"`
}

func chaveDeModelo(chave db.IntegrationApiKey) chaveResponse {
	return chaveResponse{
		ID:    chave.ID.String(),
		Label: chave.Label,
		// O valor nunca volta: o banco tem só o hash. O que a tela mostra é o
		// prefixo com os últimos quatro caracteres, que basta para identificar
		// qual chave é qual antes de revogar a errada.
		Prefix:     chave.KeyPrefix,
		Masked:     integrations.MaskKey(chave.KeyPrefix, chave.LastFour),
		Revoked:    chave.RevokedAt.Valid,
		RevokedAt:  instanteOuNil(chave.RevokedAt),
		Reason:     chave.RevokedReason.String,
		ExpiresAt:  instanteOuNil(chave.ExpiresAt),
		LastUsedAt: instanteOuNil(chave.LastUsedAt),
		LastUsedIP: chave.LastUsedIp.String,
		CreatedAt:  chave.CreatedAt,
	}
}

func (s *Server) listIntegrationKeys(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	chaves, err := s.store.ListIntegrationAPIKeys(ctx.Request.Context(), integracao.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar chaves")))
		return
	}

	itens := make([]chaveResponse, 0, len(chaves))
	for _, chave := range chaves {
		itens = append(itens, chaveDeModelo(chave))
	}
	ctx.JSON(http.StatusOK, gin.H{"api_keys": itens})
}

type criarChaveRequest struct {
	Label string `json:"label" binding:"omitempty,max=120"`
	// ExpiresInDays é opcional. Uma chave com validade é o que permite entregar
	// acesso temporário (uma prova de conceito, um fornecedor) sem depender de
	// alguém lembrar de revogar depois.
	ExpiresInDays *int `json:"expires_in_days" binding:"omitempty,min=1,max=3650"`
	// RevokeOthers é o "regerar": a chave nova entra e as anteriores caem na
	// mesma operação.
	RevokeOthers bool `json:"revoke_others"`
}

// createIntegrationKey emite uma chave nova.
//
// Com `revoke_others`, é a operação de REGERAR: a chave antiga deixa de valer
// no mesmo instante. Sem ele, a integração passa a ter duas chaves válidas —
// que é como se troca a credencial de um sistema em produção sem derrubá-lo
// (sobe a nova, atualiza o cliente, revoga a velha).
func (s *Server) createIntegrationKey(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	var req criarChaveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rotulo := strings.TrimSpace(req.Label)
	if rotulo == "" {
		rotulo = "chave " + time.Now().Format("02/01/2006 15:04")
	}

	chave, valorEmClaro, err := s.emitirChave(ctx, integracao.ID, rotulo, req.ExpiresInDays)
	if err != nil {
		log.Printf("admin: falha ao emitir chave para %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao gerar a chave")))
		return
	}

	revogadas := false
	if req.RevokeOthers {
		if err := s.store.RevokeOtherIntegrationAPIKeys(ctx.Request.Context(), db.RevokeOtherIntegrationAPIKeysParams{
			IntegrationID: integracao.ID,
			KeepID:        chave.ID,
			Reason:        pgtype.Text{String: "substituída por uma chave nova", Valid: true},
		}); err != nil {
			log.Printf("admin: falha ao revogar chaves antigas de %s: %v", integracao.ID, err)
		} else {
			revogadas = true
		}
	}

	admin, _ := currentUser(ctx)
	log.Printf("admin: chave emitida integracao=%s chave=%s prefixo=%s revogou_antigas=%t por=%s",
		integracao.ID, chave.ID, chave.KeyPrefix, revogadas, admin.ID)

	ctx.JSON(http.StatusCreated, gin.H{
		"api_key": chaveDeModelo(chave),
		// Aparece uma vez. Depois desta resposta, não existe mais nenhuma
		// forma de recuperar o valor — só emitir outra chave.
		"api_key_plaintext":     valorEmClaro,
		"revoked_previous_keys": revogadas,
	})
}

// revokeIntegrationKey invalida uma chave específica.
func (s *Server) revokeIntegrationKey(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	chaveID, err := uuid.Parse(ctx.Param("keyId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador de chave inválido")))
		return
	}

	// A conferência de dono é o que impede revogar, por um identificador
	// adivinhado, a chave de OUTRA integração.
	existente, err := s.store.GetIntegrationAPIKey(ctx.Request.Context(), chaveID)
	if err != nil || existente.IntegrationID != integracao.ID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("chave não encontrada")))
		return
	}

	motivo := strings.TrimSpace(ctx.Query("reason"))
	if motivo == "" {
		motivo = "revogada no painel"
	}

	chave, err := s.store.RevokeIntegrationAPIKey(ctx.Request.Context(), db.RevokeIntegrationAPIKeyParams{
		ID:     chaveID,
		Reason: pgtype.Text{String: motivo, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao revogar a chave")))
		return
	}

	admin, _ := currentUser(ctx)
	log.Printf("admin: chave revogada integracao=%s chave=%s por=%s", integracao.ID, chaveID, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"api_key": chaveDeModelo(chave)})
}

// emitirChave gera, grava e devolve a chave com o valor em claro.
func (s *Server) emitirChave(
	ctx *gin.Context, integrationID uuid.UUID, rotulo string, validadeDias *int,
) (db.IntegrationApiKey, string, error) {
	nova, err := integrations.GenerateKey()
	if err != nil {
		return db.IntegrationApiKey{}, "", err
	}

	var expiraEm pgtype.Timestamptz
	if validadeDias != nil {
		expiraEm = pgtype.Timestamptz{
			Time:  time.Now().AddDate(0, 0, *validadeDias),
			Valid: true,
		}
	}

	admin, _ := currentUser(ctx)

	chave, err := s.store.CreateIntegrationAPIKey(ctx.Request.Context(), db.CreateIntegrationAPIKeyParams{
		ID:            utils.GenerateUUID(),
		IntegrationID: integrationID,
		Label:         rotulo,
		KeyPrefix:     nova.Prefix,
		KeyHash:       nova.Hash,
		LastFour:      nova.LastFour,
		ExpiresAt:     expiraEm,
		CreatedBy:     pgtype.UUID{Bytes: admin.ID, Valid: admin.ID != uuid.Nil},
	})
	if err != nil {
		return db.IntegrationApiKey{}, "", err
	}
	return chave, nova.Plaintext, nil
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

type webhookResponse struct {
	ID                  string     `json:"id"`
	URL                 string     `json:"url"`
	Events              []string   `json:"events"`
	IncludeProgress     bool       `json:"include_progress"`
	Active              bool       `json:"active"`
	LastDeliveryAt      *time.Time `json:"last_delivery_at,omitempty"`
	LastStatusCode      int32      `json:"last_status_code,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	ConsecutiveFailures int32      `json:"consecutive_failures"`
	DisabledReason      string     `json:"disabled_reason,omitempty"`
	CreatedAt           *time.Time `json:"created_at"`

	// O segredo SÓ aparece para o super admin no painel, e é por isso que este
	// campo existe aqui e não na resposta da API de integrações: alguém precisa
	// poder copiá-lo para configurar a verificação do outro lado.
	Secret string `json:"secret,omitempty"`
}

func webhookDeModelo(webhook db.IntegrationWebhook, comSegredo bool) webhookResponse {
	resposta := webhookResponse{
		ID:                  webhook.ID.String(),
		URL:                 webhook.Url,
		Events:              eventosDoWebhook(webhook),
		IncludeProgress:     webhook.IncludeProgress,
		Active:              webhook.Active,
		LastDeliveryAt:      instanteOuNil(webhook.LastDeliveryAt),
		LastStatusCode:      webhook.LastStatusCode.Int32,
		LastError:           webhook.LastError.String,
		ConsecutiveFailures: webhook.ConsecutiveFailures,
		DisabledReason:      webhook.DisabledReason.String,
		CreatedAt:           webhook.CreatedAt,
	}
	if comSegredo {
		resposta.Secret = webhook.Secret
	}
	return resposta
}

func (s *Server) listAdminWebhooks(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	webhooks, err := s.store.ListIntegrationWebhooks(ctx.Request.Context(), integracao.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar webhooks")))
		return
	}

	itens := make([]webhookResponse, 0, len(webhooks))
	for _, webhook := range webhooks {
		itens = append(itens, webhookDeModelo(webhook, true))
	}
	ctx.JSON(http.StatusOK, gin.H{"webhooks": itens})
}

type criarWebhookRequest struct {
	URL             string   `json:"url" binding:"required,max=2000"`
	Events          []string `json:"events"`
	IncludeProgress bool     `json:"include_progress"`
}

func (s *Server) createAdminWebhook(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	var req criarWebhookRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// A validação da URL acontece AQUI, no cadastro, e é a mais importante
	// desta tela: é ela que impede transformar o webhook em uma sonda da nossa
	// rede interna (SSRF). A entrega revalida, mas quem erra o endereço tem de
	// descobrir agora, e não numa falha silenciosa depois.
	if err := integrations.ValidateWebhookURL(req.URL, s.config.WebhookAllowPrivate); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	eventos, err := validarEventos(req.Events)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	segredo, err := integrations.GenerateWebhookSecret()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao gerar o segredo")))
		return
	}

	webhook, err := s.store.CreateIntegrationWebhook(ctx.Request.Context(), db.CreateIntegrationWebhookParams{
		ID:              utils.GenerateUUID(),
		IntegrationID:   integracao.ID,
		Url:             strings.TrimSpace(req.URL),
		Secret:          segredo,
		Events:          eventos,
		IncludeProgress: req.IncludeProgress,
	})
	if err != nil {
		log.Printf("admin: falha ao criar webhook de %s: %v", integracao.ID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao criar webhook")))
		return
	}

	s.dispatcher.InvalidarCache(integracao.ID, integracao.UserID)

	admin, _ := currentUser(ctx)
	log.Printf("admin: webhook criado integracao=%s webhook=%s por=%s", integracao.ID, webhook.ID, admin.ID)
	ctx.JSON(http.StatusCreated, gin.H{"webhook": webhookDeModelo(webhook, true)})
}

type atualizarWebhookRequest struct {
	URL             *string   `json:"url" binding:"omitempty,max=2000"`
	Events          *[]string `json:"events"`
	IncludeProgress *bool     `json:"include_progress"`
	Active          *bool     `json:"active"`
	// RotateSecret gera um segredo novo. O anterior deixa de validar na hora,
	// então o cliente precisa atualizar a verificação dele junto.
	RotateSecret bool `json:"rotate_secret"`
}

func (s *Server) updateAdminWebhook(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}
	webhook, ok := s.webhookDaRota(ctx, integracao.ID)
	if !ok {
		return
	}

	var req atualizarWebhookRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.URL != nil {
		if err := integrations.ValidateWebhookURL(*req.URL, s.config.WebhookAllowPrivate); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}

	var eventos []string
	if req.Events != nil {
		validados, err := validarEventos(*req.Events)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		if validados == nil {
			validados = []string{}
		}
		eventos = validados
	}

	var segredo pgtype.Text
	if req.RotateSecret {
		novo, err := integrations.GenerateWebhookSecret()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao gerar o segredo")))
			return
		}
		segredo = pgtype.Text{String: novo, Valid: true}
	}

	atualizado, err := s.store.UpdateIntegrationWebhook(ctx.Request.Context(), db.UpdateIntegrationWebhookParams{
		ID:              webhook.ID,
		Url:             db.ToPgText(req.URL),
		Events:          eventos,
		IncludeProgress: db.ToPgBool(req.IncludeProgress),
		Active:          db.ToPgBool(req.Active),
		Secret:          segredo,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao atualizar webhook")))
		return
	}

	s.dispatcher.InvalidarCache(integracao.ID, integracao.UserID)

	admin, _ := currentUser(ctx)
	log.Printf("admin: webhook atualizado webhook=%s rotacionou_segredo=%t por=%s",
		atualizado.ID, req.RotateSecret, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"webhook": webhookDeModelo(atualizado, true)})
}

func (s *Server) deleteAdminWebhook(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}
	webhook, ok := s.webhookDaRota(ctx, integracao.ID)
	if !ok {
		return
	}

	if err := s.store.SoftDeleteIntegrationWebhook(ctx.Request.Context(), webhook.ID); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao remover webhook")))
		return
	}

	s.dispatcher.InvalidarCache(integracao.ID, integracao.UserID)
	ctx.JSON(http.StatusOK, gin.H{"message": "webhook removido"})
}

// testAdminWebhook dispara um evento de teste a partir do painel.
func (s *Server) testAdminWebhook(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}
	webhook, ok := s.webhookDaRota(ctx, integracao.ID)
	if !ok {
		return
	}

	if err := s.dispatcher.Enfileirar(ctx.Request.Context(), webhook, integracao.ID,
		integrations.EventPing, integrations.DownloadPayload{}); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao enfileirar o teste")))
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{
		"message": "evento de teste enfileirado; o resultado aparece na aba de entregas",
	})
}

// ---------------------------------------------------------------------------
// Entregas e auditoria
// ---------------------------------------------------------------------------

func (s *Server) listAdminDeliveries(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	limit, offset, page, perPage := paginacao(ctx)
	status, valido := filtroStatusEntrega(ctx)
	if !valido {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("status inválido")))
		return
	}

	total, err := s.store.CountWebhookDeliveries(ctx.Request.Context(), db.CountWebhookDeliveriesParams{
		IntegrationID: integracao.ID, Status: status, EventType: filtroTexto(ctx, "event_type"),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar entregas")))
		return
	}

	linhas, err := s.store.ListWebhookDeliveries(ctx.Request.Context(), db.ListWebhookDeliveriesParams{
		IntegrationID: integracao.ID, Limit: int32(limit), Offset: int32(offset),
		Status: status, EventType: filtroTexto(ctx, "event_type"),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar entregas")))
		return
	}

	itens := make([]gin.H, 0, len(linhas))
	for _, linha := range linhas {
		itens = append(itens, corpoEntrega(linha))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"deliveries": itens, "total": total, "page": page, "per_page": perPage,
		"next_page": int64(offset+len(linhas)) < total, "prev_page": page > 1,
	})
}

// retryDelivery reenfileira uma entrega que falhou.
//
// Reenvia o PAYLOAD GRAVADO, e não um remontado a partir do estado atual: o
// cliente está reconstruindo uma linha do tempo, e receber "concluído" onde o
// evento original dizia "em andamento" produziria uma história que não
// aconteceu.
func (s *Server) retryDelivery(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	entregaID, err := uuid.Parse(ctx.Param("deliveryId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	entrega, err := s.store.GetWebhookDelivery(ctx.Request.Context(), entregaID)
	if err != nil || entrega.IntegrationID != integracao.ID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("entrega não encontrada")))
		return
	}

	tarefa, err := integrations.NovaTarefaEntrega(entrega.ID.String())
	if err == nil {
		_, err = s.queueClient.EnqueueContext(ctx.Request.Context(), tarefa,
			asynq.Queue(integrations.FilaWebhook),
			asynq.MaxRetry(integrations.MaxTentativasEntrega),
			asynq.Timeout(integrations.TimeoutEntrega))
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao reenfileirar")))
		return
	}

	admin, _ := currentUser(ctx)
	log.Printf("admin: entrega reenfileirada entrega=%s por=%s", entrega.ID, admin.ID)
	ctx.JSON(http.StatusAccepted, gin.H{"message": "entrega reenfileirada"})
}

// listIntegrationRequests é a auditoria: quem chamou, de onde, o que respondemos.
func (s *Server) listIntegrationRequests(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	limit, offset, page, perPage := paginacao(ctx)

	var classe pgtype.Int4
	if bruto := strings.TrimSpace(ctx.Query("status_class")); bruto != "" {
		valor, err := strconv.Atoi(bruto)
		if err != nil || valor < 1 || valor > 5 {
			ctx.JSON(http.StatusBadRequest, errorResponse(
				errors.New("status_class deve ser 2, 4 ou 5")))
			return
		}
		classe = pgtype.Int4{Int32: int32(valor), Valid: true}
	}

	filtros := db.ListIntegrationRequestsParams{
		IntegrationID: integracao.ID,
		Limit:         int32(limit),
		Offset:        int32(offset),
		Ip:            filtroTexto(ctx, "ip"),
		StatusClass:   classe,
		Path:          filtroTexto(ctx, "path"),
		ErrorCode:     filtroTexto(ctx, "error_code"),
	}

	total, err := s.store.CountIntegrationRequests(ctx.Request.Context(), db.CountIntegrationRequestsParams{
		IntegrationID: filtros.IntegrationID, Ip: filtros.Ip,
		StatusClass: filtros.StatusClass, Path: filtros.Path, ErrorCode: filtros.ErrorCode,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar requisições")))
		return
	}

	linhas, err := s.store.ListIntegrationRequests(ctx.Request.Context(), filtros)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar requisições")))
		return
	}

	itens := make([]gin.H, 0, len(linhas))
	for _, linha := range linhas {
		item := gin.H{
			"id":          linha.ID,
			"method":      linha.Method,
			"path":        linha.Path,
			"status_code": linha.StatusCode,
			"ip":          linha.Ip,
			"user_agent":  linha.UserAgent,
			"duration_ms": linha.DurationMs,
			"error_code":  linha.ErrorCode.String,
			"created_at":  linha.CreatedAt,
		}
		if linha.ApiKeyID.Valid {
			item["api_key_id"] = uuid.UUID(linha.ApiKeyID.Bytes).String()
		}
		if linha.DownloadID.Valid {
			item["download_id"] = uuid.UUID(linha.DownloadID.Bytes).String()
		}
		itens = append(itens, item)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"requests": itens, "total": total, "page": page, "per_page": perPage,
		"next_page": int64(offset+len(linhas)) < total, "prev_page": page > 1,
	})
}

// integrationTraffic responde as duas perguntas de quem desconfia do volume:
// de quais IPs vem, e em que está dando errado.
func (s *Server) integrationTraffic(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	janela := janelaHoras(ctx)

	ips, err := s.store.IntegrationTopIPs(ctx.Request.Context(), db.IntegrationTopIPsParams{
		IntegrationID: integracao.ID, WindowHours: janela, Limit: 20,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar origens")))
		return
	}

	erros, err := s.store.IntegrationTopErrors(ctx.Request.Context(), db.IntegrationTopErrorsParams{
		IntegrationID: integracao.ID, WindowHours: janela, Limit: 20,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar erros")))
		return
	}

	origens := make([]gin.H, 0, len(ips))
	for _, linha := range ips {
		origens = append(origens, gin.H{
			"ip":        linha.Ip,
			"requests":  linha.Requests,
			"errors":    linha.Errors,
			"last_seen": linha.LastSeen,
			// Diz se aquele IP passaria pela lista atual. É o que transforma a
			// tabela em ação: o administrador vê a origem real e decide se ela
			// entra na lista de permitidos.
			"allowed": integrations.IPAutorizado(integracao.AllowedIps, linha.Ip),
		})
	}

	falhas := make([]gin.H, 0, len(erros))
	for _, linha := range erros {
		falhas = append(falhas, gin.H{
			"error_code":  linha.ErrorCode,
			"status_code": linha.StatusCode,
			"occurrences": linha.Occurrences,
			"last_seen":   linha.LastSeen,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"window_hours": janela,
		"top_ips":      origens,
		"top_errors":   falhas,
	})
}

// listIntegrationDownloadsAdmin mostra os downloads daquela integração.
func (s *Server) listIntegrationDownloadsAdmin(ctx *gin.Context) {
	integracao, ok := s.integracaoDaRota(ctx)
	if !ok {
		return
	}

	limit, offset, page, perPage := paginacao(ctx)
	status, valido := filtroStatusDownload(ctx)
	if !valido {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("status inválido")))
		return
	}

	filtros := db.ListIntegrationDownloadsParams{
		UserID: integracao.UserID, Limit: int32(limit), Offset: int32(offset),
		Status: status, Platform: filtroTexto(ctx, "platform"), Search: filtroTexto(ctx, "search"),
	}

	total, err := s.store.CountIntegrationDownloads(ctx.Request.Context(), db.CountIntegrationDownloadsParams{
		UserID: filtros.UserID, Status: filtros.Status,
		Platform: filtros.Platform, Search: filtros.Search,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar downloads")))
		return
	}

	linhas, err := s.store.ListIntegrationDownloads(ctx.Request.Context(), filtros)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar downloads")))
		return
	}

	// O painel é do super admin: aqui o detalhe técnico da falha PODE sair, e é
	// o que permite diagnosticar o download de um cliente sem abrir o container.
	itens := make([]downloadResponse, 0, len(linhas))
	for _, linha := range linhas {
		itens = append(itens, montarDownloadResponse(linha, true))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"downloads": itens, "total": total, "page": page, "per_page": perPage,
		"next_page": int64(offset+len(linhas)) < total, "prev_page": page > 1,
	})
}

// ---------------------------------------------------------------------------
// Auxiliares
// ---------------------------------------------------------------------------

// integracaoDaRota carrega a integração do parâmetro :id.
func (s *Server) integracaoDaRota(ctx *gin.Context) (db.Integration, bool) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return db.Integration{}, false
	}

	integracao, err := s.store.GetIntegrationByID(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("integração não encontrada")))
			return db.Integration{}, false
		}
		log.Printf("admin: falha ao carregar integração %s: %v", id, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar integração")))
		return db.Integration{}, false
	}
	return integracao, true
}

// webhookDaRota carrega o webhook do parâmetro :webhookId, conferindo o dono.
func (s *Server) webhookDaRota(ctx *gin.Context, integrationID uuid.UUID) (db.IntegrationWebhook, bool) {
	id, err := uuid.Parse(ctx.Param("webhookId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador de webhook inválido")))
		return db.IntegrationWebhook{}, false
	}

	webhook, err := s.store.GetIntegrationWebhook(ctx.Request.Context(), id)
	if err != nil || webhook.IntegrationID != integrationID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("webhook não encontrado")))
		return db.IntegrationWebhook{}, false
	}
	return webhook, true
}

// validarEventos recusa evento desconhecido em vez de guardá-lo.
//
// Um "download.complete" (sem o d) guardado sem reclamação produz um webhook
// que nunca dispara, e a investigação disso começa no lugar errado — no nosso
// despachante, não no cadastro.
func validarEventos(eventos []string) ([]string, error) {
	limpos := make([]string, 0, len(eventos))
	vistos := make(map[string]bool, len(eventos))

	for _, evento := range eventos {
		evento = strings.TrimSpace(evento)
		if evento == "" {
			continue
		}
		if !integrations.EventoValido(evento) {
			return nil, fmt.Errorf("evento desconhecido: %q", evento)
		}
		if vistos[evento] {
			continue
		}
		vistos[evento] = true
		limpos = append(limpos, evento)
	}
	return limpos, nil
}

// janelaHoras normaliza o período das métricas.
func janelaHoras(ctx *gin.Context) int32 {
	bruto := strings.TrimSpace(ctx.Query("window_hours"))
	if bruto == "" {
		return janelaPadraoHoras
	}
	valor, err := strconv.Atoi(bruto)
	if err != nil || valor < 1 {
		return janelaPadraoHoras
	}
	// Teto de 90 dias: uma janela sem limite faria a tela varrer a tabela de
	// auditoria inteira a cada carregamento.
	if valor > 24*90 {
		valor = 24 * 90
	}
	return int32(valor)
}

func textoOuNulo(valor string) pgtype.Text {
	valor = strings.TrimSpace(valor)
	return pgtype.Text{String: valor, Valid: valor != ""}
}

func int8OuNulo(valor *int64) pgtype.Int8 {
	if valor == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *valor, Valid: true}
}

// valorOuPadrao aplica o padrão quando o campo não veio no JSON.
func valorOuPadrao[T int32 | int64](valor *T, padrao T) T {
	if valor == nil {
		return padrao
	}
	return *valor
}

func restanteNaoNegativo(valor int64) int64 {
	if valor < 0 {
		return 0
	}
	return valor
}
