package server

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

type browserSessionResponse struct {
	Account youtubeAccountResponse `json:"account"`
	Ticket  string                 `json:"ticket"`
	// WSPath é relativo à própria API: o frontend só precisa trocar o esquema
	// para ws/wss. O serviço de navegador nunca é endereçado pelo navegador do
	// administrador.
	WSPath      string `json:"ws_path"`
	VNCPassword string `json:"vnc_password,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ExpiresIn   int    `json:"ticket_expires_in_seconds"`
}

// openYoutubeBrowser abre (ou reaproveita) o navegador remoto da conta e emite
// o ticket de acesso. É idempotente: dois cliques não criam dois navegadores.
func (s *Server) openYoutubeBrowser(ctx *gin.Context) {
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
	if !account.Active {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("conta desativada")))
		return
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), browserOperationTimeout)
	defer cancel()

	session, err := s.browser.StartSession(requestCtx, accountID)
	if err != nil {
		log.Printf("admin: falha ao abrir navegador account_id=%s: %v", accountID, err)
		switch {
		case errors.Is(err, browser.ErrTooManySessions):
			// Condição operacional: o administrador precisa saber que basta
			// fechar outro navegador.
			ctx.JSON(http.StatusTooManyRequests, errorResponse(
				errors.New("limite de navegadores simultâneos atingido; feche outro navegador antes de abrir este")))
		case errors.Is(err, browser.ErrSessionUnavailable):
			ctx.JSON(http.StatusConflict, errorResponse(err))
		default:
			ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("não foi possível abrir o navegador remoto")))
		}
		return
	}

	// Uma conta que ainda não tinha perfil passa a aguardar o login manual. Os
	// demais estados são preservados: só o health check decide autenticação.
	if account.Status == db.CoreYoutubeAccountStatusNOTCONFIGURED {
		if updated, err := s.store.UpdateYoutubeAccountStatus(ctx.Request.Context(), db.UpdateYoutubeAccountStatusParams{
			ID:     accountID,
			Status: db.CoreYoutubeAccountStatusAWAITINGLOGIN,
		}); err == nil {
			account = updated
		}
	}

	user, _ := currentUser(ctx)
	ticket, err := s.issueBrowserTicket(ctx.Request.Context(), user.ID, accountID)
	if err != nil {
		log.Printf("admin: falha ao emitir ticket account_id=%s: %v", accountID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("não foi possível autorizar o acesso ao navegador")))
		return
	}

	log.Printf("admin: navegador aberto account_id=%s por=%s", accountID, user.ID)

	ctx.JSON(http.StatusOK, browserSessionResponse{
		Account:     newYoutubeAccountResponse(account, session),
		Ticket:      ticket,
		WSPath:      "/v1/admin/youtube/accounts/" + accountID.String() + "/browser/ws",
		VNCPassword: session.VNCPassword,
		Width:       session.Width,
		Height:      session.Height,
		ExpiresIn:   int(browserTicketTTL.Seconds()),
	})
}

// issueYoutubeBrowserTicket renova o ticket sem reabrir o navegador, usado
// quando o WebSocket cai e o painel precisa reconectar.
func (s *Server) issueYoutubeBrowserTicket(ctx *gin.Context) {
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

	session := s.sessionInfo(ctx.Request.Context(), accountID)
	if session.State != browser.SessionStateRunning {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("o navegador desta conta não está aberto")))
		return
	}

	user, _ := currentUser(ctx)
	ticket, err := s.issueBrowserTicket(ctx.Request.Context(), user.ID, accountID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("não foi possível autorizar o acesso ao navegador")))
		return
	}

	ctx.JSON(http.StatusOK, browserSessionResponse{
		Account:     newYoutubeAccountResponse(account, session),
		Ticket:      ticket,
		WSPath:      "/v1/admin/youtube/accounts/" + accountID.String() + "/browser/ws",
		VNCPassword: session.VNCPassword,
		Width:       session.Width,
		Height:      session.Height,
		ExpiresIn:   int(browserTicketTTL.Seconds()),
	})
}

// closeYoutubeBrowser encerra o navegador preservando o perfil e, em seguida,
// confere a sessão: é o momento natural para o painel refletir "autenticada".
func (s *Server) closeYoutubeBrowser(ctx *gin.Context) {
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

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), browserOperationTimeout)
	defer cancel()

	if _, err := s.browser.StopSession(requestCtx, accountID); err != nil {
		log.Printf("admin: falha ao fechar navegador account_id=%s: %v", accountID, err)
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("não foi possível fechar o navegador remoto")))
		return
	}

	log.Printf("admin: navegador encerrado account_id=%s", accountID)

	updated := s.refreshAccountStatus(ctx.Request.Context(), account)
	ctx.JSON(http.StatusOK, newYoutubeAccountResponse(updated, s.sessionInfo(ctx.Request.Context(), accountID)))
}

var browserUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	// A autorização é feita pelo ticket de uso único, verificado antes do
	// upgrade. A checagem de origem fica a cargo do Traefik/CORS já existente.
	CheckOrigin: func(*http.Request) bool { return true },
}

// youtubeBrowserWS é o único caminho até a tela do navegador remoto. Ele valida
// o ticket, reconfirma o papel de super admin no banco e só então liga os dois
// WebSockets. Um usuário comum não passa daqui nem com um ticket válido, porque
// o ticket é emitido apenas para super admins e é reconferido aqui.
func (s *Server) youtubeBrowserWS(ctx *gin.Context) {
	if !s.browser.Configured() {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("serviço de navegador remoto não configurado")))
		return
	}

	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	userID, ticketAccountID, err := s.consumeBrowserTicket(ctx.Request.Context(), ctx.Query("ticket"))
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}
	if ticketAccountID != accountID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("ticket não pertence a esta conta")))
		return
	}

	user, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("usuário não encontrado")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao autorizar o acesso")))
		return
	}
	if !user.Active || user.Role != db.CoreUserRoleSuperAdmin {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("acesso negado")))
		return
	}

	if _, ok := s.loadAccount(ctx, accountID); !ok {
		return
	}

	upstream, err := s.browser.DialVNC(ctx.Request.Context(), accountID)
	if err != nil {
		log.Printf("admin: falha ao conectar no navegador account_id=%s: %v", accountID, err)
		status := http.StatusBadGateway
		if errors.Is(err, browser.ErrSessionUnavailable) {
			status = http.StatusConflict
		}
		ctx.JSON(status, errorResponse(errors.New("navegador remoto indisponível")))
		return
	}
	defer upstream.Close()

	client, err := browserUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer client.Close()

	log.Printf("admin: sessão de navegador conectada account_id=%s por=%s", accountID, user.ID)
	defer log.Printf("admin: sessão de navegador desconectada account_id=%s por=%s", accountID, user.ID)

	pipeWebSockets(client, upstream)
}

// pipeWebSockets encaminha os quadros RFB nos dois sentidos até que um dos
// lados feche.
func pipeWebSockets(client, upstream *websocket.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		copyWebSocket(upstream, client)
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		copyWebSocket(client, upstream)
	}()

	<-done
	_ = client.SetReadDeadline(time.Now())
	_ = upstream.SetReadDeadline(time.Now())
}

func copyWebSocket(destination, source *websocket.Conn) {
	for {
		messageType, reader, err := source.NextReader()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
			continue
		}

		writer, err := destination.NextWriter(messageType)
		if err != nil {
			return
		}
		if _, err := io.Copy(writer, reader); err != nil {
			_ = writer.Close()
			return
		}
		if err := writer.Close(); err != nil {
			return
		}
	}
}
