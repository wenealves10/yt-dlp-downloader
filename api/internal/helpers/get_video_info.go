package helpers

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"

	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
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

func GetVideoInfo(ctx context.Context, url string) (*VideoInfo, error) {
	var cmd *exec.Cmd
	if configs.LoadedConfig.ProxyEnabled {
		proxyUrl := configs.LoadedConfig.ProxyURL
		cmd = exec.CommandContext(ctx, "yt-dlp", "--proxy", proxyUrl, "--dump-json", url)
	} else {
		cmd = exec.CommandContext(ctx,
			"yt-dlp",
			"--cookies", configs.LoadedConfig.YoutubeDLFileCookies,
			"--user-agent", configs.LoadedConfig.YoutubeDLUserAgent,
			"--referer", configs.LoadedConfig.YoutubeDLReferer,
			"--add-header", "DNT: 1",
			"--dump-json", url)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var info VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, err
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

	return &info, nil
}
