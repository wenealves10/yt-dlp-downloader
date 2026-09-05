package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

// browserOperationTimeout cobre abrir o navegador, que envolve subir Xvfb,
// Chrome e o servidor VNC.
const browserOperationTimeout = 90 * time.Second

// youtubeAccountResponse é a visão que o painel recebe. Deliberadamente não
// existe nenhum campo de cookie, token ou senha: o front nunca vê material de
// sessão.
type youtubeAccountResponse struct {
	ID                  string     `json:"id"`
	Label               string     `json:"label"`
	Email               string     `json:"email,omitempty"`
	Status              string     `json:"status"`
	BrowserState        string     `json:"browser_state"`
	Active              bool       `json:"active"`
	Priority            int32      `json:"priority"`
	Viewers             int        `json:"viewers"`
	LastError           string     `json:"last_error,omitempty"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	LastAuthenticatedAt *time.Time `json:"last_authenticated_at,omitempty"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	// NextInRotation marca a conta que o próximo download vai escolher.
	NextInRotation    bool       `json:"next_in_rotation"`
	BrowserStartedAt  *time.Time `json:"browser_started_at,omitempty"`
	BrowserActivityAt *time.Time `json:"browser_activity_at,omitempty"`
	CreatedAt         *time.Time `json:"created_at"`
}

func newYoutubeAccountResponse(account db.YoutubeAccount, session browser.SessionInfo) youtubeAccountResponse {
	response := youtubeAccountResponse{
		ID:                account.ID.String(),
		Label:             account.Label,
		Email:             account.Email.String,
		Status:            string(account.Status),
		BrowserState:      string(browser.SessionStateStopped),
		Active:            account.Active,
		Priority:          account.Priority,
		LastError:         account.LastError.String,
		BrowserStartedAt:  session.StartedAt,
		BrowserActivityAt: session.LastActivity,
		Viewers:           session.Viewers,
		CreatedAt:         account.CreatedAt,
	}

	if session.State != "" {
		response.BrowserState = string(session.State)
	}
	if account.LastCheckedAt.Valid {
		checkedAt := account.LastCheckedAt.Time
		response.LastCheckedAt = &checkedAt
	}
	if account.LastAuthenticatedAt.Valid {
		authenticatedAt := account.LastAuthenticatedAt.Time
		response.LastAuthenticatedAt = &authenticatedAt
	}
	if account.LastUsedAt.Valid {
		usedAt := account.LastUsedAt.Time
		response.LastUsedAt = &usedAt
	}

	return response
}

// requireBrowserService centraliza a checagem da integração para não repetir a
// mesma resposta em todo handler.
func (s *Server) requireBrowserService(ctx *gin.Context) bool {
	if s.browser.Configured() {
		return true
	}
	ctx.JSON(http.StatusServiceUnavailable, errorResponse(
		errors.New("serviço de navegador remoto não está configurado neste ambiente")))
	return false
}

// parseAccountID valida o identificador antes de qualquer uso, o que também
// impede que um valor arbitrário chegue ao caminho de perfil no serviço de
// navegador.
func parseAccountID(ctx *gin.Context) (uuid.UUID, bool) {
	accountID, err := uuid.Parse(ctx.Param("id"))
	if err != nil || accountID == uuid.Nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador de conta inválido")))
		return uuid.Nil, false
	}
	return accountID, true
}

func (s *Server) loadAccount(ctx *gin.Context, accountID uuid.UUID) (db.YoutubeAccount, bool) {
	account, err := s.store.GetYoutubeAccountByID(ctx.Request.Context(), accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("conta não encontrada")))
			return db.YoutubeAccount{}, false
		}
		log.Printf("admin: falha ao carregar conta account_id=%s: %v", accountID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar a conta")))
		return db.YoutubeAccount{}, false
	}
	return account, true
}

// sessionInfo consulta o estado runtime sem transformar a indisponibilidade do
// serviço em erro de listagem: uma conta continua visível mesmo com o serviço
// de navegador fora do ar.
func (s *Server) sessionInfo(ctx context.Context, accountID uuid.UUID) browser.SessionInfo {
	if !s.browser.Configured() {
		return browser.SessionInfo{AccountID: accountID.String(), State: browser.SessionStateStopped}
	}

	info, err := s.browser.SessionInfo(ctx, accountID)
	if err != nil {
		log.Printf("admin: estado do navegador indisponível account_id=%s: %v", accountID, err)
		return browser.SessionInfo{AccountID: accountID.String(), State: browser.SessionStateStopped}
	}
	return info
}

func (s *Server) listYoutubeAccounts(ctx *gin.Context) {
	accounts, err := s.store.GetYoutubeAccounts(ctx.Request.Context())
	if err != nil {
		log.Printf("admin: falha ao listar contas: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar contas")))
		return
	}

	sessions := map[string]browser.SessionInfo{}
	browserAvailable := s.browser.Configured()
	if browserAvailable {
		fetched, err := s.browser.Sessions(ctx.Request.Context())
		if err != nil {
			log.Printf("admin: serviço de navegador indisponível: %v", err)
			browserAvailable = false
		} else {
			sessions = fetched
		}
	}

	response := make([]youtubeAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		response = append(response, newYoutubeAccountResponse(account, sessions[account.ID.String()]))
	}
	markNextInRotation(response, accounts)

	// authenticated_count mostra quantas contas o rodízio tem à disposição no
	// momento; é o que responde "o downloader ainda tem para onde ir?".
	authenticated, err := s.store.CountAuthenticatedYoutubeAccounts(ctx.Request.Context())
	if err != nil {
		log.Printf("admin: falha ao contar contas autenticadas: %v", err)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"accounts":            response,
		"browser_available":   browserAvailable,
		"authenticated_count": authenticated,
	})
}

func (s *Server) getYoutubeAccount(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	account, ok := s.loadAccount(ctx, accountID)
	if !ok {
		return
	}

	ctx.JSON(http.StatusOK, newYoutubeAccountResponse(account, s.sessionInfo(ctx.Request.Context(), accountID)))
}

type createYoutubeAccountRequest struct {
	Label    string `json:"label" binding:"required,min=2,max=80"`
	Email    string `json:"email" binding:"omitempty,email"`
	Priority int32  `json:"priority" binding:"omitempty,min=1,max=1000"`
}

// createYoutubeAccount cadastra a conta e prepara o perfil isolado. O navegador
// não é aberto aqui: quem decide isso é o administrador, na ação seguinte.
func (s *Server) createYoutubeAccount(ctx *gin.Context) {
	if !s.requireBrowserService(ctx) {
		return
	}

	var req createYoutubeAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	user, _ := currentUser(ctx)
	accountID := utils.GenerateUUID()

	status := db.CoreYoutubeAccountStatusAWAITINGLOGIN
	if _, err := s.browser.EnsureProfile(ctx.Request.Context(), accountID); err != nil {
		log.Printf("admin: falha ao preparar perfil account_id=%s: %v", accountID, err)
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("não foi possível preparar a sessão da conta")))
		return
	}

	priority := req.Priority
	if priority == 0 {
		priority = 100
	}

	account, err := s.store.CreateYoutubeAccount(ctx.Request.Context(), db.CreateYoutubeAccountParams{
		ID:         accountID,
		Label:      req.Label,
		Email:      pgtype.Text{String: req.Email, Valid: req.Email != ""},
		Status:     status,
		ProfileDir: accountID.String(),
		Priority:   priority,
		CreatedBy:  db.ToPgUUID(user.ID),
	})
	if err != nil {
		// O perfil recém-criado não pode ficar órfão se a linha não gravar.
		if cleanupErr := s.browser.DeleteProfile(ctx.Request.Context(), accountID); cleanupErr != nil {
			log.Printf("admin: falha ao limpar perfil órfão account_id=%s: %v", accountID, cleanupErr)
		}
		log.Printf("admin: falha ao criar conta: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New(db.HandleDBError(err))))
		return
	}

	log.Printf("admin: conta criada account_id=%s label=%s por=%s", account.ID, account.Label, user.ID)
	ctx.JSON(http.StatusCreated, newYoutubeAccountResponse(account, browser.SessionInfo{State: browser.SessionStateStopped}))
}

type updateYoutubeAccountRequest struct {
	Label    *string `json:"label" binding:"omitempty,min=2,max=80"`
	Email    *string `json:"email" binding:"omitempty,email"`
	Active   *bool   `json:"active"`
	Priority *int32  `json:"priority" binding:"omitempty,min=1,max=1000"`
}

func (s *Server) updateYoutubeAccount(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	var req updateYoutubeAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, ok := s.loadAccount(ctx, accountID); !ok {
		return
	}

	account, err := s.store.UpdateYoutubeAccount(ctx.Request.Context(), db.UpdateYoutubeAccountParams{
		ID:       accountID,
		Label:    db.ToPgText(req.Label),
		Email:    db.ToPgText(req.Email),
		Active:   db.ToPgBool(req.Active),
		Priority: db.ToPgInt4(req.Priority),
	})
	if err != nil {
		log.Printf("admin: falha ao atualizar conta account_id=%s: %v", accountID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao atualizar a conta")))
		return
	}

	ctx.JSON(http.StatusOK, newYoutubeAccountResponse(account, s.sessionInfo(ctx.Request.Context(), accountID)))
}

// deleteYoutubeAccount encerra o navegador, apaga o perfil persistente e marca
// a conta como removida.
func (s *Server) deleteYoutubeAccount(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	if _, ok := s.loadAccount(ctx, accountID); !ok {
		return
	}

	if s.browser.Configured() {
		requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), browserOperationTimeout)
		defer cancel()

		if err := s.browser.DeleteProfile(requestCtx, accountID); err != nil {
			log.Printf("admin: falha ao remover perfil account_id=%s: %v", accountID, err)
			ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("não foi possível remover a sessão da conta")))
			return
		}
	}

	if err := s.store.DeleteYoutubeAccount(ctx.Request.Context(), accountID); err != nil {
		log.Printf("admin: falha ao remover conta account_id=%s: %v", accountID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao remover a conta")))
		return
	}

	log.Printf("admin: conta removida account_id=%s", accountID)
	ctx.JSON(http.StatusOK, gin.H{"message": "conta removida"})
}

// checkYoutubeAccount roda o health check sob demanda e grava o veredito.
func (s *Server) checkYoutubeAccount(ctx *gin.Context) {
	if !s.requireBrowserService(ctx) {
		return
	}

	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	account, ok := s.loadAccount(ctx, accountID)
	if !ok {
		return
	}

	updated := s.refreshAccountStatus(ctx.Request.Context(), account)
	ctx.JSON(http.StatusOK, newYoutubeAccountResponse(updated, s.sessionInfo(ctx.Request.Context(), accountID)))
}

// refreshAccountStatus consulta o serviço de navegador e persiste o resultado.
// Devolve a conta já atualizada, ou a original quando o check falha.
func (s *Server) refreshAccountStatus(ctx context.Context, account db.YoutubeAccount) db.YoutubeAccount {
	requestCtx, cancel := context.WithTimeout(ctx, browserOperationTimeout)
	defer cancel()

	result, err := s.browser.Check(requestCtx, account.ID)

	status := db.CoreYoutubeAccountStatusAUTHENTICATED
	lastError := pgtype.Text{Valid: false}

	switch {
	case err != nil:
		status = db.CoreYoutubeAccountStatusERROR
		lastError = pgtype.Text{String: truncateMessage(browser.UserMessage(err)), Valid: true}
		log.Printf("admin: health check falhou account_id=%s: %v", account.ID, err)
	case !result.Authenticated:
		status = db.CoreYoutubeAccountStatusREQUIRESAUTH
		lastError = pgtype.Text{String: truncateMessage(result.Reason), Valid: result.Reason != ""}
		log.Printf("admin: sessão inválida account_id=%s motivo=%s", account.ID, result.Reason)
	default:
		log.Printf("admin: sessão válida account_id=%s", account.ID)
	}

	updated, updateErr := s.store.UpdateYoutubeAccountStatus(ctx, db.UpdateYoutubeAccountStatusParams{
		ID:        account.ID,
		Status:    status,
		LastError: lastError,
	})
	if updateErr != nil {
		log.Printf("admin: falha ao gravar status account_id=%s: %v", account.ID, updateErr)
		return account
	}

	if err == nil && result.Email != "" && result.Email != updated.Email.String {
		withEmail, emailErr := s.store.UpdateYoutubeAccount(ctx, db.UpdateYoutubeAccountParams{
			ID:    account.ID,
			Email: pgtype.Text{String: result.Email, Valid: true},
		})
		if emailErr == nil {
			return withEmail
		}
		log.Printf("admin: falha ao gravar e-mail da conta account_id=%s: %v", account.ID, emailErr)
	}

	return updated
}

func truncateMessage(message string) string {
	const limit = 500
	if len(message) <= limit {
		return message
	}
	return message[:limit] + "…"
}

// markNextInRotation reproduz a mesma ordem usada pelo banco para escolher a
// conta: menor prioridade, depois uso mais antigo. Serve só para o painel
// mostrar quem é a próxima; a escolha de verdade continua sendo atômica no
// banco, no momento do download.
func markNextInRotation(response []youtubeAccountResponse, accounts []db.YoutubeAccount) {
	best := -1

	for i, account := range accounts {
		if !account.Active || account.Status != db.CoreYoutubeAccountStatusAUTHENTICATED {
			continue
		}
		if best < 0 || rotatesBefore(account, accounts[best]) {
			best = i
		}
	}

	if best >= 0 && best < len(response) {
		response[best].NextInRotation = true
	}
}

func rotatesBefore(candidate, current db.YoutubeAccount) bool {
	if candidate.Priority != current.Priority {
		return candidate.Priority < current.Priority
	}
	// Nunca usada vem antes de qualquer uma que já foi usada.
	if candidate.LastUsedAt.Valid != current.LastUsedAt.Valid {
		return !candidate.LastUsedAt.Valid
	}
	if candidate.LastUsedAt.Valid && !candidate.LastUsedAt.Time.Equal(current.LastUsedAt.Time) {
		return candidate.LastUsedAt.Time.Before(current.LastUsedAt.Time)
	}
	if candidate.CreatedAt != nil && current.CreatedAt != nil {
		return candidate.CreatedAt.Before(*current.CreatedAt)
	}
	return false
}
