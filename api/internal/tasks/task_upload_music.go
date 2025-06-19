package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type UploadMusicPayload struct {
	DownloadID string `json:"download_id"`
	MusicPath  string `json:"music_path"`
	Filename   string `json:"filename"`
}

func NewUploadMusicTask(musicPath, downloadID, filename string) (*asynq.Task, error) {
	payload, err := json.Marshal(UploadMusicPayload{
		DownloadID: downloadID,
		MusicPath:  musicPath,
		Filename:   filename,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUploadMusic, payload), nil
}
