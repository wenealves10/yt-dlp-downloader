package queues

const (
	TypeDownloadVideoQueue      = "download_video"
	TypeDownloadMusicQueue      = "download_music"
	TypeUploadVideoQueue        = "upload_video"
	TypeUploadMusicQueue        = "upload_music"
	TypeResumeVideoQueue        = "resume_video"
	TypeResumeMusicQueue        = "resume_music"
	TypeCreateTweetVideoQueue   = "create_tweet_video"
	TypeCreateTweetMusicQueue   = "create_tweet_music"
	TypeDownloadExpirationQueue = "download_expiration"
	TypeDeleteDownloadQueue     = "delete_download"
	TypeYoutubeHealthCheckQueue = "youtube_health_check"
	TypeDownloadMediaQueue      = "download_media"
	TypeTempCleanupQueue        = "temp_cleanup"
	TypeMediaHealthQueue        = "media_health"
)

const (
	// WorkerConcurrency is the global ceiling for a single worker process.
	// Tasks above this limit remain queued in Redis until a slot is available.
	WorkerConcurrency = 20

	// Queue weights define how often each queue is selected; they do not grant
	// additional concurrency beyond WorkerConcurrency.
	QueueWeightDownloadVideo    = 30
	QueueWeightDownloadMusic    = 60
	QueueWeightUploadVideo      = 20
	QueueWeightUploadMusic      = 40
	QueueWeightResumeVideo      = 10
	QueueWeightResumeMusic      = 20
	QueueWeightCreateTweetVideo = 10
	QueueWeightCreateTweetMusic = 10
	QueueWeightFileExpiration   = 1
	QueueWeightDeleteDownload   = 10
	QueueWeightYoutubeHealth    = 1
	QueueWeightDownloadMedia    = 60
	QueueWeightTempCleanup      = 1
	QueueWeightMediaHealth      = 1
)

// MaxTentativasDownload limita quantas vezes um download é reenfileirado.
//
// O padrão do asynq é 25. Contra uma plataforma que está recusando o servidor,
// isso vira duas dezenas de tentativas seguidas contra quem já disse não — o
// tipo de insistência que aprofunda o bloqueio em vez de contorná-lo. Três
// cobre a falha passageira (uma rede que oscilou, um 5xx) e para por aí; erros
// de conteúdo já não chegam a repetir.
const MaxTentativasDownload = 3
