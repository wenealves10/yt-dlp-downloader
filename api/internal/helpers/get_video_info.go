package helpers

import (
	"context"
	"encoding/json"
	"os/exec"

	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
)

type VideoInfo struct {
	Title       string `json:"title"`
	Thumbnail   string `json:"thumbnail"`
	Description string `json:"description"`
	Duration    int    `json:"duration"` // duração em segundos
	Uploader    string `json:"uploader"`
	LikeCount   int    `json:"like_count"`
	ViewCount   int    `json:"view_count"`
	UploadDate  string `json:"upload_date"` // formato: "20240629"
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
			"--add-header", configs.LoadedConfig.YoutubeDLAddHeader,
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
	return &info, nil
}
