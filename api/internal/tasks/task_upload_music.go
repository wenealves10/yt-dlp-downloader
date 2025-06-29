package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type UploadMusicPayload struct {
	DownloadID string `json:"download_id"`
	MusicPath  string `json:"music_path"`
	BannerPath string `json:"banner_path,omitempty"`
	Filename   string `json:"filename"`
}

func NewUploadMusicTask(musicPath, downloadID, filename string, bannerPath string) (*asynq.Task, error) {
	payload, err := json.Marshal(UploadMusicPayload{
		DownloadID: downloadID,
		MusicPath:  musicPath,
		Filename:   filename,
		BannerPath: bannerPath,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUploadMusic, payload), nil
}
