package stream

import (
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

const StreamName = "downloads"
const ConsumerName = "download_consumer"
const ConsumerGroup = "download_group"

type DownloadEvent struct {
	ID              string                `json:"id"`
	UserID          string                `json:"user_id"`
	Status          db.CoreDownloadStatus `json:"status"`
	ThumbnailUrl    string                `json:"thumbnail_url,omitempty"`
	Title           string                `json:"title,omitempty"`
	OriginalUrl     string                `json:"original_url,omitempty"`
	Format          db.CoreFormatType     `json:"format,omitempty"`
	FileUrl         string                `json:"file_url,omitempty"`
	ExpiresAt       time.Time             `json:"expires_at,omitempty"`
	DurationSeconds int32                 `json:"duration_seconds,omitempty"`
	ErrorMessage    string                `json:"error_message,omitempty"`
	CreatedAt       time.Time             `json:"created_at,omitempty"`
}
