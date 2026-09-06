package stream

import (
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

const StreamName = "downloads"
const ConsumerName = "download_consumer"
const ConsumerGroup = "download_group"

// ProgressEvent é o andamento em tempo real. Vai por SSE porque o banco só
// guarda o último estado conhecido — persistir cada atualização seria escrever
// dezenas de vezes por download sem ninguém ler.
type ProgressEvent struct {
	Percent         float64 `json:"percent"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      int64   `json:"total_bytes"`
	SpeedBPS        int64   `json:"speed_bps"`
	ETASeconds      int     `json:"eta_seconds"`
	Postprocess     bool    `json:"postprocess"`
}

type DownloadEvent struct {
	ID           string                `json:"id"`
	UserID       string                `json:"user_id"`
	Status       db.CoreDownloadStatus `json:"status"`
	ThumbnailUrl string                `json:"thumbnail_url,omitempty"`
	Title        string                `json:"title,omitempty"`
	OriginalUrl  string                `json:"original_url,omitempty"`
	Format       db.CoreFormatType     `json:"format,omitempty"`
	FileUrl      string                `json:"file_url,omitempty"`
	// Ponteiro porque `omitempty` não omite um time.Time zerado: sem isso, todo
	// evento de progresso carregaria "0001-01-01" e sobrescreveria na tela a
	// data de expiração real de um download já concluído.
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	DurationSeconds int32      `json:"duration_seconds,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	Platform        string     `json:"platform,omitempty"`
	// O rótulo vem pronto do servidor: mapear plataforma para nome em dois
	// lugares faria os dois divergirem.
	PlatformLabel string         `json:"platform_label,omitempty"`
	Progress      *ProgressEvent `json:"progress,omitempty"`
}
