package jobs

import (
	"context"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
)

type JobDownloadExpiration struct {
	client *asynq.Client
	store  db.Store
}

func NewJobDownloadExpiration(client *asynq.Client, store db.Store) *JobDownloadExpiration {
	return &JobDownloadExpiration{
		client: client,
		store:  store,
	}
}

func (p *JobDownloadExpiration) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	expiredDownloads, err := p.store.GetDownloadsExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to get expired downloads: %v", err)
	}

	for _, download := range expiredDownloads {
		taskDeleteDownload, err := tasks.NewDeleteDownloadTask(download.ID.String())
		if err != nil {
			log.Printf("failed to create delete download task: %v", err)
			continue
		}

		if _, err := p.client.EnqueueContext(ctx, taskDeleteDownload, asynq.Queue(queues.TypeDeleteDownloadQueue)); err != nil {
			log.Printf("failed to enqueue delete download task for ID %s: %v", download.ID, err)
			continue
		}
	}

	return nil
}
