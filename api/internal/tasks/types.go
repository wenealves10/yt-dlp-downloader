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
)
