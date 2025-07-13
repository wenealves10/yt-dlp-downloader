package server

import (
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
	"github.com/wenealves10/yt-dlp-downloader/pkg/sse"
)

type Server struct {
	config       configs.Config
	store        db.Store
	storage      storage.Storage
	queueClient  *asynq.Client
	sseManager   *sse.SSEManager
	tokenCreator tokens.TokenCreator
	router       *gin.Engine
}

func NewServer(config configs.Config, store db.Store, queueClient *asynq.Client, sseManager *sse.SSEManager, storage storage.Storage) (*Server, error) {

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
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	router.Use(cors.New(config))

	// Set the trusted proxies to handle forwarded headers correctly
	router.SetTrustedProxies([]string{
		"127.0.0.1",      // localhost
		"10.0.0.0/8",     // IPs internos docker bridge / docker network
		"172.16.0.0/12",  // idem
		"192.168.0.0/16", // idem
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
	authRoutes.PUT("/change-password", server.updatePassword)

	// downloads routes
	authRoutes.GET("/downloads", server.getDownloads)
	authRoutes.GET("/downloads/daily", server.getDailyDownloads)
	authRoutes.DELETE("/downloads/:id", server.deleteDownload)
	authLimitedRoutes.POST("/downloads", server.createDownload)

	// SSE route
	groupV1.GET("/sse", server.sseHandler())

	server.router = router
}

func (s *Server) Start(address string) error {
	return s.router.Run(address)
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
