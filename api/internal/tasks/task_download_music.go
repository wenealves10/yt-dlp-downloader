package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type DownloadMusicPayload struct {
	MusicURL string `json:"music_url"`
}

func NewDownloadMusicTask(musicURL string) (*asynq.Task, error) {
	payload, err := json.Marshal(DownloadMusicPayload{
		MusicURL: musicURL,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDownloadMusic, payload), nil
}
