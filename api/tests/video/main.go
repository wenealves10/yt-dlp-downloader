package main

import (
	"context"
	"log"

	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/helpers"
)

func main() {
	_, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}
	url_video := "https://www.youtube.com/watch?v=HWjoQ92VKEs"
	ctx := context.Background()
	videoInfo, err := helpers.GetVideoInfo(ctx, url_video)
	if err != nil {
		log.Fatalf("Error getting video info: %v", err)
	}
	imagemBannerLocal := "./uploads/videos/banners/banner.jpg"

	err = helpers.DownloadImage(ctx, videoInfo.Thumbnail, imagemBannerLocal)
	if err != nil {
		log.Fatalf("Error downloading video thumbnail: %v", err)
	}

	log.Printf("Video Title: %s", videoInfo.Title)
	log.Printf("Video Thumbnail: %s", videoInfo.Thumbnail)
	log.Printf("Video Description: %s", videoInfo.Description)
	log.Printf("Video Duration: %d seconds", videoInfo.Duration)
	log.Printf("Video Uploader: %s", videoInfo.Uploader)
	log.Printf("Video Like Count: %d", videoInfo.LikeCount)
	log.Printf("Video View Count: %d", videoInfo.ViewCount)
	log.Printf("Video Upload Date: %s", videoInfo.UploadDate)
}
