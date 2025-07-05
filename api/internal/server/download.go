package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/helpers"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

// Types for download requests and responses

type DownloadType string

const (
	DownloadTypeVideo DownloadType = "video"
	DownloadTypeMusic DownloadType = "music"
)

type downloadRequest struct {
	Type DownloadType `json:"type"  binding:"required,oneof=video music"`
	URL  string       `json:"url"  binding:"required,url,valid_url"`
}

type downloadResponse struct {
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	OriginalUrl     string                `json:"original_url"`
	Format          db.CoreFormatType     `json:"format"`
	Status          db.CoreDownloadStatus `json:"status"`
	ThumbnailUrl    string                `json:"thumbnail_url,omitempty"`
	FileUrl         string                `json:"file_url,omitempty"`
	ExpiresAt       string                `json:"expires_at,omitempty"`
	DurationSeconds int32                 `json:"duration_seconds,omitempty"`
	ErrorMessage    string                `json:"error_message,omitempty"`
	CreatedAt       string                `json:"created_at"`
}

func (s *Server) createDownload(ctx *gin.Context) {
	var req downloadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid request format"})
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		log.Printf("Failed to parse user ID: %v", err)
		ctx.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	videoInfo, err := helpers.GetVideoInfo(ctx, req.URL)
	if err != nil {
		log.Printf("Failed to get video info: %v", err)
		ctx.JSON(400, gin.H{"error": "Invalid URL or unsupported format"})
		return
	}

	downloadParams := db.CreateDownloadParams{
		ID:          utils.GenerateUUID(),
		UserID:      userID,
		OriginalUrl: req.URL,
		Title:       videoInfo.Title,
		ThumbnailUrl: pgtype.Text{
			String: videoInfo.Thumbnail,
			Valid:  videoInfo.Thumbnail != "",
		},
		DurationSeconds: pgtype.Int4{
			Int32: int32(videoInfo.Duration),
			Valid: true,
		},
		Status: db.CoreDownloadStatusPENDING,
	}

	switch req.Type {
	case DownloadTypeVideo:
		downloadParams.Format = db.CoreFormatTypeMP4 // Default format for video
	case DownloadTypeMusic:
		downloadParams.Format = db.CoreFormatTypeMP3 // Default format for music
	}

	download, err := s.store.CreateDownload(ctx, downloadParams)
	if err != nil {
		log.Printf("Failed to create download: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to create download"})
		return
	}

	if req.Type == DownloadTypeVideo {
		if task, err := tasks.NewDownloadVideoTask(download.ID.String()); err == nil {
			if info, err := s.queueClient.Enqueue(task, asynq.Queue(queues.TypeDownloadVideoQueue)); err == nil {
				log.Printf("Enqueued task video: ID=%s, Type=%s, Queue=%s", info.ID, info.Type, info.Queue)
			} else {
				log.Printf("failed to enqueue video task: %v", err)
			}
		} else {
			log.Printf("failed to create video task: %v", err)
		}
	}

	if req.Type == DownloadTypeMusic {
		if task, err := tasks.NewDownloadMusicTask(download.ID.String()); err == nil {
			if info, err := s.queueClient.Enqueue(task, asynq.Queue(queues.TypeDownloadMusicQueue)); err == nil {
				log.Printf("Enqueued task music: ID=%s, Type=%s, Queue=%s", info.ID, info.Type, info.Queue)
			} else {
				log.Printf("failed to enqueue music task: %v", err)
			}
		} else {
			log.Printf("failed to create music task: %v", err)
		}
	}

	response := downloadResponse{
		ID:          download.ID.String(),
		Status:      download.Status,
		OriginalUrl: download.OriginalUrl,
		Format:      download.Format,
		CreatedAt:   download.CreatedAt.Format(time.RFC3339),
	}

	ctx.JSON(http.StatusOK, response)
}

func (s *Server) getDownloads(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		log.Printf("Failed to parse user ID: %v", err)
		ctx.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	pageStr := ctx.DefaultQuery("page", "1")
	perPageStr := ctx.DefaultQuery("perPage", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(perPageStr)
	if err != nil || perPage < 1 {
		perPage = 10
	}

	offset := (page - 1) * perPage

	totalDownloads, err := s.store.CountDownloadsByUser(ctx, userID)
	if err != nil {
		log.Printf("Failed to count downloads: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to count downloads"})
		return
	}

	downloads, err := s.store.GetDownloadsByUser(ctx, db.GetDownloadsByUserParams{
		UserID: userID,
		Limit:  int32(perPage),
		Offset: int32(offset),
	})
	if err != nil {
		log.Printf("Failed to get downloads: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to get downloads"})
		return
	}

	var response []downloadResponse
	for _, download := range downloads {
		response = append(response, downloadResponse{
			ID:              download.ID.String(),
			Title:           download.Title,
			Status:          download.Status,
			OriginalUrl:     download.OriginalUrl,
			ThumbnailUrl:    download.ThumbnailUrl.String,
			FileUrl:         download.FileUrl.String,
			DurationSeconds: download.DurationSeconds.Int32,
			ExpiresAt:       download.ExpiresAt.Time.Format(time.RFC3339),
			ErrorMessage:    download.ErrorMessage.String,
			Format:          download.Format,
			CreatedAt:       download.CreatedAt.Format(time.RFC3339),
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"next_page": (page * perPage) < int(totalDownloads),
		"prev_page": page > 1,
		"page":      page,
		"per_page":  perPage,
		"total":     totalDownloads,
		"downloads": response,
	})
}

func (s *Server) getDailyDownloads(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		log.Printf("Failed to parse user ID: %v", err)
		ctx.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		log.Printf("Failed to get user: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to get user"})
		return
	}

	dailyDownloads, err := s.store.CountDownloadsToday(ctx, userID)
	if err != nil {
		log.Printf("Failed to count daily downloads: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to count daily downloads"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"daily_downloads": dailyDownloads,
		"daily_limit":     user.DailyLimit,
		"remaining":       int64(user.DailyLimit) - dailyDownloads,
	})
}

func (s *Server) deleteDownload(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		log.Printf("Failed to parse user ID: %v", err)
		ctx.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	downloadID := ctx.Param("id")
	if downloadID == "" {
		ctx.JSON(400, gin.H{"error": "Download ID is required"})
		return
	}

	downloadIDParsed, err := utils.ParseUUID(downloadID)
	if err != nil {
		log.Printf("Failed to parse download ID: %v", err)
		ctx.JSON(400, gin.H{"error": "Invalid download ID"})
		return
	}

	downloadExists, err := s.store.GetDownloadByID(ctx, downloadIDParsed)
	if err != nil {
		log.Printf("Not found download: %v\n", err)
		ctx.JSON(404, gin.H{"error": "Download not found"})
		return
	}

	if downloadExists.UserID != userID {
		log.Printf("Unauthorized access to download: %s by user: %s", downloadID, userID)
		ctx.JSON(403, gin.H{"error": "You do not have permission to delete this download"})
		return
	}

	if err := s.store.DeleteDownload(ctx, downloadIDParsed); err != nil {
		log.Printf("Failed to delete download: %v", err)
		ctx.JSON(500, gin.H{"error": "Failed to delete download"})
		return
	}

	if err := s.storage.DeleteFile(ctx, downloadExists.ThumbnailUrl.String); err != nil {
		log.Printf("Failed to delete thumbnail from storage: %v", err)
	}

	if err := s.storage.DeleteFile(ctx, downloadExists.FileUrl.String); err != nil {
		log.Printf("Failed to delete file from storage: %v", err)
	}

	ctx.JSON(http.StatusNoContent, gin.H{})
}
