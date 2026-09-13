package tasks

const (
	TypeDownloadVideo      = "download:video"
	TypeDownloadMusic      = "download:music"
	TypeUploadVideo        = "upload:video"
	TypeUploadMusic        = "upload:music"
	TypeResumeVideo        = "resume:video"
	TypeResumeMusic        = "resume:music"
	TypeCreateTweetVideo   = "create:tweet:video"
	TypeCreateTweetMusic   = "create:tweet:music"
	TypeDownloadExpiration = "download:expiration"
	TypeDeleteDownload     = "download:delete"
	TypeYoutubeHealthCheck = "youtube:session:health"
	// TypeDownloadMedia substitui os dois tipos específicos acima. Eles seguem
	// registrados para as tarefas que já estavam na fila quando a versão nova
	// subiu.
	TypeDownloadMedia = "download:media"
	TypeTempCleanup   = "media:temp:cleanup"
	TypeMediaHealth   = "media:health:report"
	// Notificação de um sistema integrado. A entrega depende de um servidor de
	// terceiro responder, e por isso roda no worker: um endpoint lento não pode
	// ocupar quem atende as requisições HTTP.
	TypeIntegrationWebhook = "integration:webhook:deliver"
	// Poda das tabelas de auditoria das integrações, que crescem a cada
	// chamada e a cada notificação.
	TypeIntegrationPrune = "integration:logs:prune"
)
