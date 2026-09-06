package browserd

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
)

// authHeader carrega o segredo compartilhado entre API, worker e este serviço.
// O serviço não publica portas: só é alcançável pela rede interna do Docker, e
// mesmo lá exige o token.
const authHeader = "X-Browser-Token"

// Server expõe o Manager via HTTP para a API e o worker.
type Server struct {
	manager *Manager
	token   string
	router  *gin.Engine
}

func NewServer(manager *Manager, token string) *Server {
	server := &Server{manager: manager, token: token}
	server.setupRouter()
	return server
}

func (s *Server) setupRouter() {
	router := gin.New()
	router.Use(gin.Recovery())

	// O logger padrão do Gin registra apenas método, rota e status: nenhum
	// corpo de resposta (e portanto nenhum cookie) chega ao log.
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health"}}))

	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	internal := router.Group("/internal", s.authMiddleware())
	internal.GET("/sessions", s.listSessions)
	internal.GET("/sessions/:id", s.getSession)
	internal.POST("/sessions/:id/start", s.startSession)
	internal.POST("/sessions/:id/stop", s.stopSession)
	internal.POST("/sessions/:id/profile", s.createProfile)
	internal.DELETE("/sessions/:id/profile", s.deleteProfile)
	internal.POST("/sessions/:id/check", s.checkSession)
	internal.GET("/sessions/:id/cookies", s.sessionCookies)
	internal.GET("/sessions/:id/vnc", s.vncBridge)

	s.router = router
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	expected := []byte(s.token)

	return func(ctx *gin.Context) {
		provided := []byte(ctx.GetHeader(authHeader))
		if len(expected) == 0 || subtle.ConstantTimeCompare(expected, provided) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, browser.ErrorResponse{Error: "não autorizado"})
			return
		}
		ctx.Next()
	}
}

// respondError traduz os erros do Manager em códigos HTTP, sem vazar caminhos
// de arquivo ou detalhes internos.
func respondError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidAccountID):
		ctx.JSON(http.StatusBadRequest, browser.ErrorResponse{Error: "identificador de conta inválido"})
	case errors.Is(err, ErrSessionNotRunning):
		ctx.JSON(http.StatusConflict, browser.ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrTooManySessions):
		ctx.JSON(http.StatusTooManyRequests, browser.ErrorResponse{Error: err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, browser.ErrorResponse{Error: err.Error()})
	}
}

func (s *Server) listSessions(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, s.manager.List())
}

func (s *Server) getSession(ctx *gin.Context) {
	info, err := s.manager.Info(ctx.Param("id"))
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, info)
}

// plataformaDa lê a plataforma da conta da query. O banco é a fonte da verdade
// e fica do outro lado (na API), então ela viaja no pedido. Ausente cai no
// padrão: durante um deploy em etapas a API antiga ainda chama sem o parâmetro,
// e todas as contas dela são do YouTube.
func plataformaDa(ctx *gin.Context) string {
	plataforma := ctx.Query("platform")
	if !browser.PlataformaSuportada(plataforma) {
		return browser.PlataformaPadrao
	}
	return plataforma
}

func (s *Server) startSession(ctx *gin.Context) {
	info, err := s.manager.Start(ctx.Param("id"), plataformaDa(ctx))
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, info)
}

func (s *Server) stopSession(ctx *gin.Context) {
	err := s.manager.Stop(ctx.Param("id"))
	if err != nil && !errors.Is(err, ErrSessionNotRunning) {
		respondError(ctx, err)
		return
	}

	info, err := s.manager.Info(ctx.Param("id"))
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, info)
}

func (s *Server) createProfile(ctx *gin.Context) {
	info, err := s.manager.EnsureProfile(ctx.Param("id"))
	if err != nil {
		respondError(ctx, err)
		return
	}
	log.Printf("browserd: perfil preparado account_id=%s", info.AccountID)
	ctx.JSON(http.StatusOK, info)
}

func (s *Server) deleteProfile(ctx *gin.Context) {
	if err := s.manager.DeleteProfile(ctx.Param("id")); err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "perfil removido"})
}

func (s *Server) checkSession(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 90*time.Second)
	defer cancel()

	result, err := s.manager.Check(requestCtx, ctx.Param("id"), plataformaDa(ctx))
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// sessionCookies entrega o jar em texto puro. É a única rota que expõe dados de
// sessão e existe apenas para o worker montar o arquivo temporário do yt-dlp.
func (s *Server) sessionCookies(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 90*time.Second)
	defer cancel()

	jar, err := s.manager.Cookies(requestCtx, ctx.Param("id"), plataformaDa(ctx))
	if err != nil {
		respondError(ctx, err)
		return
	}

	ctx.Header("Cache-Control", "no-store")
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", jar)
}

var vncUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	// A autorização acontece no header X-Browser-Token, verificado antes deste
	// handler. O único cliente possível é a API, que não envia Origin.
	CheckOrigin: func(*http.Request) bool { return true },
}

// vncBridge liga o WebSocket ao servidor RFB local. O x11vnc escuta apenas em
// 127.0.0.1, então esta ponte é o único caminho até a tela do navegador — e ela
// já passou pelo token do serviço e, antes disso, pela autorização de super
// admin na API.
func (s *Server) vncBridge(ctx *gin.Context) {
	accountID := ctx.Param("id")

	port, _, err := s.manager.AddViewer(accountID)
	if err != nil {
		respondError(ctx, err)
		return
	}
	defer s.manager.RemoveViewer(accountID)

	tcpConn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, browser.ErrorResponse{Error: "servidor gráfico indisponível"})
		return
	}
	defer tcpConn.Close()

	wsConn, err := vncUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	log.Printf("browserd: visualizador conectado account_id=%s", accountID)
	defer log.Printf("browserd: visualizador desconectado account_id=%s", accountID)

	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		buffer := make([]byte, 32*1024)
		for {
			read, err := tcpConn.Read(buffer)
			if read > 0 {
				if err := wsConn.WriteMessage(websocket.BinaryMessage, buffer[:read]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			messageType, reader, err := wsConn.NextReader()
			if err != nil {
				return
			}
			if messageType != websocket.BinaryMessage {
				continue
			}
			if _, err := io.Copy(tcpConn, reader); err != nil {
				return
			}
		}
	}()

	<-done
}
