package stream

import (
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

const StreamName = "downloads"

type DownloadEvent struct {
	ID              string                `json:"id"`
	UserID          string                `json:"user_id"`
	Status          db.CoreDownloadStatus `json:"status"`
	ThumbnailUrl    string                `json:"thumbnail_url,omitempty"`
	FileUrl         string                `json:"file_url,omitempty"`
	ExpiresAt       time.Time             `json:"expires_at,omitempty"`
	DurationSeconds int32                 `json:"duration_seconds,omitempty"`
	ErrorMessage    string                `json:"error_message,omitempty"`
}
