package queues

const (
	TypeDownloadVideoQueue    = "download_video"
	TypeDownloadMusicQueue    = "download_music"
	TypeResumeVideoQueue      = "resume_video"
	TypeResumeMusicQueue      = "resume_music"
	TypeCreateTweetVideoQueue = "create_tweet_video"
	TypeCreateTweetMusicQueue = "create_tweet_music"
)

const (
	Concurrency                 = 100
	ConcurrencyDownloadVideo    = 30
	ConcurrencyDownloadMusic    = 60
	ConcurrencyResumeVideo      = 10
	ConcurrencyResumeMusic      = 20
	ConcurrencyCreateTweetVideo = 10
	ConcurrencyCreateTweetMusic = 10
)
