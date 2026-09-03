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
)
