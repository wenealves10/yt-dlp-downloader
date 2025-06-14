package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type UploadMusicPayload struct {
	MusicPath string `json:"music_path"`
	Filename  string `json:"filename"`
}

func NewUploadMusicTask(musicPath, filename string) (*asynq.Task, error) {
	payload, err := json.Marshal(UploadMusicPayload{
		MusicPath: musicPath,
		Filename:  filename,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUploadMusic, payload), nil
}
