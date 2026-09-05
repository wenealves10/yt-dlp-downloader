package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
	"github.com/wenealves10/yt-dlp-downloader/internal/ytaccounts"
	"github.com/wenealves10/yt-dlp-downloader/pkg/sse"
)

type Server struct {
	config       configs.Config
	store        db.Store
	storage      storage.Storage
	queueClient  *asynq.Client
	sseManager   *sse.SSEManager
	tokenCreator tokens.TokenCreator
	redis        *redis.Client
	browser      *browser.Client
	accounts     ytaccounts.Provider
	router       *gin.Engine
}

func NewServer(
	config configs.Config,
	store db.Store,
	queueClient *asynq.Client,
	sseManager *sse.SSEManager,
	storage storage.Storage,
	redisClient *redis.Client,
	browserClient *browser.Client,
	accounts ytaccounts.Provider,
) (*Server, error) {

	tokenCreator, err := tokens.NewPasetoTokenCreator(config.TokenPasetoKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token creator: %w", err)
	}

	server := &Server{
		store:        store,
		tokenCreator: tokenCreator,
		config:       config,
		queueClient:  queueClient,
		sseManager:   sseManager,
		storage:      storage,
		redis:        redisClient,
		browser:      browserClient,
		accounts:     accounts,
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("valid_url", ValidYouTubeURL)
	}

	server.setupRouter()
	return server, nil
}

func (server *Server) setupRouter() {
	router := gin.Default()

	// Set up middleware for logging and recovery
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Set up CORS middleware
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	router.Use(cors.New(config))

	// Set the trusted proxies to handle forwarded headers correctly
	router.SetTrustedProxies([]string{
		"127.0.0.1",      // localhost
		"10.0.0.0/8",     // IPs internos docker bridge / docker network
		"172.16.0.0/12",  // idem
		"192.168.0.0/16", // idem
	})

	// Sonda de saúde do orquestrador. Sem ela o Swarm marca a réplica como
	// pronta no instante em que o processo nasce — antes do pool do Postgres —
	// e o Traefik manda tráfego para quem ainda não atende. Não expõe nada:
	// apenas confirma que o processo responde e que o banco está alcançável.
	router.GET("/healthz", func(ctx *gin.Context) {
		checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), 3*time.Second)
		defer cancel()

		if _, err := server.store.CountYoutubeAccounts(checkCtx); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	groupV1 := router.Group("/v1")

	groupV1.POST("/auth/register", server.register)
	groupV1.POST("/auth/login", server.login)
	// router.POST("/auth/forgot-password", server.forgotPassword)
	// router.POST("/auth/reset-password", server.resetPassword)
	// router.GET("/auth/verify-email", server.verifyEmail)

	authRoutes := groupV1.Group("/").Use(authMiddleware(server.tokenCreator, server.store))
	authLimitedRoutes := groupV1.Group("/").Use(authMiddleware(server.tokenCreator, server.store), limitMiddleware(server.store))

	// user routes
	authRoutes.GET("/profile", server.getProfile)
	authRoutes.PATCH("/profile", server.updateProfile)
	authRoutes.PUT("/profile/change-password", server.updatePassword)

	// downloads routes
	authRoutes.GET("/downloads", server.getDownloads)
	authRoutes.GET("/downloads/daily", server.getDailyDownloads)
	authRoutes.GET("/downloads/:id/file", server.downloadFile)
	authRoutes.GET("/downloads/:id/download-url", server.downloadURL)
	authRoutes.DELETE("/downloads/:id", server.deleteDownload)
	authLimitedRoutes.POST("/downloads", server.createDownload)
	// A regular browser navigation cannot include an Authorization header. The
	// form route receives the bearer token in the POST body and streams an
	// attachment, allowing Safari and other browsers to start the download
	// without navigating to the R2 public URL.
	groupV1.POST("/downloads/:id/file", formAuthMiddleware(server.tokenCreator, server.store), server.downloadFile)

	// SSE route
	groupV1.GET("/sse", server.sseHandler())

	// Painel do super admin: contas do YouTube usadas pelo downloader.
	adminRoutes := groupV1.Group("/admin", authMiddleware(server.tokenCreator, server.store), superAdminMiddleware())
	adminRoutes.GET("/youtube/accounts", server.listYoutubeAccounts)
	adminRoutes.POST("/youtube/accounts", server.createYoutubeAccount)
	adminRoutes.GET("/youtube/accounts/:id", server.getYoutubeAccount)
	adminRoutes.PATCH("/youtube/accounts/:id", server.updateYoutubeAccount)
	adminRoutes.DELETE("/youtube/accounts/:id", server.deleteYoutubeAccount)
	adminRoutes.POST("/youtube/accounts/:id/check", server.checkYoutubeAccount)
	adminRoutes.POST("/youtube/accounts/:id/browser", server.openYoutubeBrowser)
	adminRoutes.POST("/youtube/accounts/:id/browser/ticket", server.issueYoutubeBrowserTicket)
	adminRoutes.DELETE("/youtube/accounts/:id/browser", server.closeYoutubeBrowser)

	// O WebSocket do navegador remoto não passa pelo authMiddleware porque a
	// API de WebSocket do navegador não envia o header Authorization. Ele usa
	// um ticket de uso único, emitido acima somente para super admins e
	// reconferido no handler.
	groupV1.GET("/admin/youtube/accounts/:id/browser/ws", server.youtubeBrowserWS)

	server.router = router
}

func (s *Server) Start(address string) error {
	// Bootstrap do primeiro super admin, quando configurado.
	ensureSuperAdmin(context.Background(), s.store, s.config.SuperAdminEmail)
	return s.router.Run(address)
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
