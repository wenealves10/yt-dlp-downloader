package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
	"github.com/wenealves10/yt-dlp-downloader/internal/ytaccounts"
	"github.com/wenealves10/yt-dlp-downloader/pkg/sse"
)

type Server struct {
	config        configs.Config
	store         db.Store
	storage       storage.Storage
	queueClient   *asynq.Client
	sseManager    *sse.SSEManager
	tokenCreator  tokens.TokenCreator
	redis         *redis.Client
	browser       *browser.Client
	mediaRegistry *media.Registry
	accounts      ytaccounts.Provider
	// rdStream deixa a API publicar no mesmo stream que o worker usa. É o que
	// permite o cancelamento aparecer na tela na hora, sem esperar o worker
	// notar o pedido.
	rdStream stream.EventPublisher

	// Peças da seção de integrações. Nenhuma delas é obrigatória para o resto
	// da API funcionar: sem Redis o limitador falha aberto, e sem elas as rotas
	// de integração simplesmente não são exercitadas.
	limiter    *integrations.Limiter
	auditor    *auditor
	dispatcher *integrations.Dispatcher

	router *gin.Engine
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
	mediaRegistry *media.Registry,
) (*Server, error) {

	tokenCreator, err := tokens.NewPasetoTokenCreator(config.TokenPasetoKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token creator: %w", err)
	}

	server := &Server{
		store:         store,
		tokenCreator:  tokenCreator,
		config:        config,
		queueClient:   queueClient,
		sseManager:    sseManager,
		storage:       storage,
		redis:         redisClient,
		browser:       browserClient,
		accounts:      accounts,
		mediaRegistry: mediaRegistry,
	}

	if redisClient != nil {
		server.rdStream = stream.NewRedisPublisher(redisClient)
	}

	// A seção de integrações é montada aqui, e não em main.go, porque tudo de
	// que ela precisa (store, fila, Redis, config) o servidor já tem. Pedir
	// esses componentes de novo na assinatura de NewServer só criaria uma
	// segunda chance de passar um deles diferente.
	server.limiter = integrations.NewLimiter(redisClient)
	server.auditor = novoAuditor(store)
	server.dispatcher = integrations.NewDispatcher(store, queueClient, server.limiter,
		integrations.DispatcherConfig{
			BaseURL:            strings.TrimRight(config.PublicAPIURL, "/"),
			IntervaloProgresso: config.WebhookProgressInterval,
		})

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

	// Download multiplataforma. A resolução não conta contra o limite diário:
	// é consulta de metadados, não download.
	authRoutes.POST("/media/resolve", server.resolveMedia)
	authLimitedRoutes.POST("/media/downloads", server.createMediaDownload)
	authRoutes.POST("/downloads/:id/cancel", server.cancelDownload)

	// SSE route
	groupV1.GET("/sse", server.sseHandler())

	// Painel do super admin.
	adminRoutes := groupV1.Group("/admin", authMiddleware(server.tokenCreator, server.store), superAdminMiddleware())

	// Métricas e histórico global.
	adminRoutes.GET("/overview", server.overview)
	adminRoutes.GET("/providers", server.listProviders)
	adminRoutes.GET("/downloads", server.listDownloads)
	adminRoutes.DELETE("/downloads/:id", server.deleteDownloadAdmin)

	// Usuários.
	adminRoutes.GET("/users", server.listUsers)
	adminRoutes.POST("/users", server.createUser)
	adminRoutes.GET("/users/:id", server.getUser)
	adminRoutes.PATCH("/users/:id", server.updateUser)
	adminRoutes.POST("/users/:id/password", server.resetUserPassword)
	adminRoutes.DELETE("/users/:id", server.deleteUser)

	// Contas do YouTube usadas pelo downloader.
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

	// ---------------------------------------------------------------------
	// Integrações — painel do super admin
	// ---------------------------------------------------------------------
	//
	// Criar integração, emitir e revogar chave, cadastrar webhook e ler a
	// auditoria é SEMPRE operação de pessoa autenticada no painel. Não existe
	// auto-cadastro: um sistema que pudesse criar o próprio acesso tornaria a
	// cota e a lista de IPs decorativas.
	adminRoutes.GET("/integrations", server.listIntegrations)
	adminRoutes.POST("/integrations", server.createIntegration)
	adminRoutes.GET("/integrations/:id", server.getIntegration)
	adminRoutes.PATCH("/integrations/:id", server.updateIntegration)
	adminRoutes.DELETE("/integrations/:id", server.deleteIntegration)

	adminRoutes.GET("/integrations/:id/keys", server.listIntegrationKeys)
	adminRoutes.POST("/integrations/:id/keys", server.createIntegrationKey)
	adminRoutes.DELETE("/integrations/:id/keys/:keyId", server.revokeIntegrationKey)

	adminRoutes.GET("/integrations/:id/webhooks", server.listAdminWebhooks)
	adminRoutes.POST("/integrations/:id/webhooks", server.createAdminWebhook)
	adminRoutes.PATCH("/integrations/:id/webhooks/:webhookId", server.updateAdminWebhook)
	adminRoutes.DELETE("/integrations/:id/webhooks/:webhookId", server.deleteAdminWebhook)
	adminRoutes.POST("/integrations/:id/webhooks/:webhookId/test", server.testAdminWebhook)

	adminRoutes.GET("/integrations/:id/deliveries", server.listAdminDeliveries)
	adminRoutes.POST("/integrations/:id/deliveries/:deliveryId/retry", server.retryDelivery)
	adminRoutes.GET("/integrations/:id/requests", server.listIntegrationRequests)
	adminRoutes.GET("/integrations/:id/traffic", server.integrationTraffic)
	adminRoutes.GET("/integrations/:id/downloads", server.listIntegrationDownloadsAdmin)

	// ---------------------------------------------------------------------
	// Integrações — a API que os outros sistemas consomem
	// ---------------------------------------------------------------------
	//
	// A ordem dos middlewares é a do raciocínio, e importa:
	//
	//  1. recuperação própria, para que um pânico responda JSON com `code` em
	//     vez do corpo vazio do Recovery global;
	//  2. auditoria, que é a MAIS EXTERNA das duas restantes justamente para
	//     enxergar também o que a autenticação recusou — as tentativas
	//     recusadas são as que interessam numa investigação;
	//  3. autenticação por chave de API, que aplica IP autorizado e limite de
	//     chamadas.
	//
	// As rotas de download são os MESMOS handlers da tela. A conta de serviço
	// entra no contexto pela mesma chave que o login usa, e é isso que permite
	// reaproveitá-los sem duplicar a lógica de formato, sessão e cancelamento.
	integrationRoutes := groupV1.Group("/integration",
		integrationRecovery(),
		server.integrationAuditMiddleware(),
		server.integrationAuthMiddleware(),
	)

	integrationRoutes.GET("/me", server.integrationMe)
	integrationRoutes.GET("/quota", server.integrationQuota)

	// Resolver metadados não cria download e não consome cota: é a consulta que
	// antecede a escolha da qualidade, igual à da tela.
	integrationRoutes.POST("/media/resolve", server.resolveMedia)

	// Só a criação passa pelo controle de cota e de simultâneos. Aplicá-lo na
	// consulta faria um cliente sem cota perder também a capacidade de saber o
	// que já pediu — e aí ele repetiria os pedidos, que é o oposto do objetivo.
	integrationRoutes.POST("/downloads",
		server.integrationQuotaMiddleware(), server.integrationCreateDownload)

	integrationRoutes.GET("/downloads", server.integrationListDownloads)
	integrationRoutes.GET("/downloads/:id", server.integrationGetDownload)
	integrationRoutes.POST("/downloads/:id/cancel", server.integrationCancelDownload)
	integrationRoutes.DELETE("/downloads/:id", server.deleteDownload)
	integrationRoutes.GET("/downloads/:id/download-url", server.downloadURL)
	integrationRoutes.GET("/downloads/:id/file", server.downloadFile)

	integrationRoutes.GET("/webhooks", server.integrationListWebhooks)
	integrationRoutes.POST("/webhooks/:webhookId/test", server.integrationTestWebhook)
	integrationRoutes.GET("/deliveries", server.integrationListDeliveries)

	// Rota inexistente sob /v1/integration responde no mesmo formato das
	// outras. O 404 padrão do gin vem sem corpo, e seria a única resposta desta
	// API que o cliente não conseguiria tratar programaticamente.
	router.NoRoute(func(ctx *gin.Context) {
		if strings.HasPrefix(ctx.Request.URL.Path, "/v1/integration") {
			erroNaoEncontrado(ctx)
			return
		}
		ctx.JSON(http.StatusNotFound, gin.H{"error": "rota não encontrada"})
	})

	// Documentação da API, servida pelo próprio serviço: uma especificação em
	// arquivo solto no repositório envelhece sem ninguém notar, e esta é a
	// mesma que o código publica.
	if server.config.DocsEnabled {
		server.registrarDocumentacao(router)
	}

	server.router = router
}

func (s *Server) Start(address string) error {
	// Bootstrap do primeiro super admin, quando configurado.
	ctx := context.Background()
	ensureSuperAdmin(ctx, s.store, s.config.SuperAdminEmail)

	// Os dois trabalham fora do caminho da requisição: o auditor grava o log de
	// acesso das integrações, e o despachante transforma evento de download em
	// entrega de webhook.
	s.auditor.iniciar(ctx)
	s.dispatcher.Start(ctx)

	return s.router.Run(address)
}

// Dispatcher expõe o despachante para quem consome o stream de eventos.
//
// O consumidor vive em main.go (é ele que também alimenta o SSE), e é de lá que
// cada evento é entregue aqui. Um segundo consumidor só para webhooks criaria um
// grupo de consumo concorrente, e metade dos eventos deixaria de chegar à tela.
func (s *Server) Dispatcher() *integrations.Dispatcher {
	return s.dispatcher
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
