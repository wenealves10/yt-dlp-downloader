package queues

const (
	TypeDownloadVideoQueue    = "download_video"
	TypeDownloadMusicQueue    = "download_music"
	TypeUploadVideoQueue      = "upload_video"
	TypeUploadMusicQueue      = "upload_music"
	TypeResumeVideoQueue      = "resume_video"
	TypeResumeMusicQueue      = "resume_music"
	TypeCreateTweetVideoQueue = "create_tweet_video"
	TypeCreateTweetMusicQueue = "create_tweet_music"
)

const (
	Concurrency                 = 100
	ConcurrencyDownloadVideo    = 30
	ConcurrencyDownloadMusic    = 60
	ConcurrencyUploadVideo      = 20
	ConcurrencyUploadMusic      = 40
	ConcurrencyResumeVideo      = 10
	ConcurrencyResumeMusic      = 20
	ConcurrencyCreateTweetVideo = 10
	ConcurrencyCreateTweetMusic = 10
)
