package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"sort"

	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/ytaccounts"
)

type Format struct {
	FormatID string `json:"format_id"`
	Filesize int64  `json:"filesize"`
	Ext      string `json:"ext"`
}

type VideoInfo struct {
	Title         string   `json:"title"`
	Thumbnail     string   `json:"thumbnail"`
	Description   string   `json:"description"`
	Duration      int      `json:"duration"` // duração em segundos
	Uploader      string   `json:"uploader"`
	LikeCount     int      `json:"like_count"`
	ViewCount     int      `json:"view_count"`
	UploadDate    string   `json:"upload_date"` // formato: "20240629"
	FileSizeBytes int64    `json:"filesize_bytes"`
	Formats       []Format `json:"formats"`
}

// maxInfoAttempts permite trocar de conta uma vez quando a primeira sessão é
// recusada pelo YouTube. Mais que isso apenas atrasaria a resposta ao usuário.
const maxInfoAttempts = 2

// GetVideoInfo consulta os metadados do vídeo. Quando existe uma conta
// gerenciada autenticada, a consulta usa a sessão dela; caso contrário segue
// com a configuração estática de cookies.
//
// Se o YouTube recusar a sessão, a conta é retirada do rodízio e a consulta é
// repetida com a próxima conta disponível.
func GetVideoInfo(ctx context.Context, provider ytaccounts.Provider, url string) (*VideoInfo, error) {
	var lastErr error

	for attempt := 0; attempt < maxInfoAttempts; attempt++ {
		info, stderr, err := dumpVideoJSON(ctx, provider, url)
		if err == nil {
			return info, nil
		}
		lastErr = err

		// Só vale tentar de novo se a falha foi de autenticação: qualquer
		// outro erro se repetiria com outra conta.
		if !ytaccounts.IsAuthFailure(stderr) {
			break
		}
	}

	return nil, lastErr
}

// dumpVideoJSON executa uma tentativa e devolve também o stderr do yt-dlp, que
// é onde a recusa de sessão aparece.
func dumpVideoJSON(ctx context.Context, provider ytaccounts.Provider, url string) (*VideoInfo, []byte, error) {
	lease := ytaccounts.Acquire(ctx, provider, browser.PlataformaPadrao)
	defer lease.Release()

	args := append(ytaccounts.AuthArgs(lease), "--dump-json", url)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

	output, err := cmd.Output()
	if err != nil {
		var stderr []byte
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}
		ytaccounts.ReportAuthFailure(ctx, provider, lease, stderr)
		return nil, stderr, err
	}

	var info VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, nil, err
	}

	var formatsWithSize []Format
	for _, f := range info.Formats {
		if f.Filesize > 0 {
			formatsWithSize = append(formatsWithSize, f)
		}
	}

	// Sort from largest to smallest
	sort.Slice(formatsWithSize, func(i, j int) bool {
		return formatsWithSize[i].Filesize > formatsWithSize[j].Filesize
	})

	var sizeBytes int64
	if len(formatsWithSize) > 0 {
		sizeBytes = formatsWithSize[0].Filesize
	}

	info.FileSizeBytes = sizeBytes

	return &info, nil, nil
}
