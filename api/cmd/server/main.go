package main

import (
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
)

func main() {
	redisAddr := fmt.Sprintf("%s:%s", configs.REDIS_HOST, configs.REDIS_PORT)
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: configs.REDIS_USERNAME,
		Password: configs.REDIS_PASSWORD,
	})
	defer client.Close()

	// Enqueue a task video
	task, err := tasks.NewDownloadVideoTask("https://www.youtube.com/watch?v=HWjoQ92VKEs&ab_channel=DanX")
	if err != nil {
		log.Fatalf("failed to create task: %v", err)
	}

	// Enqueue the task video
	info, err := client.Enqueue(task, asynq.Queue(queues.TypeDownloadVideoQueue))
	if err != nil {
		log.Fatalf("failed to enqueue task: %v", err)
	}

	// Enqueue a task music
	taskMusic, err := tasks.NewDownloadMusicTask("https://www.youtube.com/watch?v=HWjoQ92VKEs&ab_channel=DanX")
	if err != nil {
		log.Fatalf("failed to create music task: %v", err)
	}

	// Enqueue the task music
	infoMusic, err := client.Enqueue(taskMusic, asynq.Queue(queues.TypeDownloadMusicQueue))
	if err != nil {
		log.Fatalf("failed to enqueue music task: %v", err)
	}
	log.Printf("Enqueued task video: ID=%s, Type=%s, Queue=%s", info.ID, info.Type, info.Queue)
	log.Printf("Enqueued task music: ID=%s, Type=%s, Queue=%s", infoMusic.ID, infoMusic.Type, infoMusic.Queue)

}
