package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

type UploadVideoPayload struct {
	VideoPath string `json:"video_path"`
	Filename  string `json:"filename"`
}

func NewUploadVideoTask(videoPath, filename string) (*asynq.Task, error) {
	payload, err := json.Marshal(UploadVideoPayload{
		VideoPath: videoPath,
		Filename:  filename,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUploadVideo, payload), nil
}
