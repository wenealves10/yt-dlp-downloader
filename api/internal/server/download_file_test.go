package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
)

type downloadStoreStub struct {
	db.Store
	download db.Download
	err      error
}

func (s downloadStoreStub) GetDownloadByID(context.Context, uuid.UUID) (db.Download, error) {
	return s.download, s.err
}

type downloadStorageStub struct {
	storage.Storage
	file      storage.FileStream
	err       error
	requested string
}

func (s *downloadStorageStub) OpenFile(_ context.Context, objectKey string) (storage.FileStream, error) {
	s.requested = objectKey
	return s.file, s.err
}

func TestDownloadFileStreamsAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	downloadID := uuid.New()
	storageStub := &downloadStorageStub{
		file: storage.FileStream{
			Body:          io.NopCloser(strings.NewReader("video bytes")),
			ContentLength: int64(len("video bytes")),
			ContentType:   "video/mp4",
		},
	}
	server := &Server{
		store: downloadStoreStub{download: db.Download{
			ID:        downloadID,
			UserID:    userID,
			Title:     "Meu vídeo / final",
			Format:    db.CoreFormatTypeMP4,
			Status:    db.CoreDownloadStatusCOMPLETED,
			FileUrl:   pgtype.Text{String: "uploads/videos/file.mp4", Valid: true},
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		}},
		storage: storageStub,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/downloads/"+downloadID.String()+"/file", nil)
	ctx.Params = gin.Params{{Key: "id", Value: downloadID.String()}}
	ctx.Set(authorizationPayloadKey, &tokens.Payload{UserID: userID.String()})

	server.downloadFile(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	require.Contains(t, recorder.Header().Get("Content-Disposition"), ".mp4")
	require.Equal(t, "video bytes", recorder.Body.String())
	require.Equal(t, "uploads/videos/file.mp4", storageStub.requested)
}

func TestDownloadFileRejectsAnotherUsersFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	downloadID := uuid.New()
	storageStub := &downloadStorageStub{}
	server := &Server{
		store: downloadStoreStub{download: db.Download{
			ID:     downloadID,
			UserID: uuid.New(),
			Status: db.CoreDownloadStatusCOMPLETED,
			FileUrl: pgtype.Text{
				String: "uploads/videos/file.mp4",
				Valid:  true,
			},
		}},
		storage: storageStub,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/downloads/"+downloadID.String()+"/file", nil)
	ctx.Params = gin.Params{{Key: "id", Value: downloadID.String()}}
	ctx.Set(authorizationPayloadKey, &tokens.Payload{UserID: uuid.New().String()})

	server.downloadFile(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Empty(t, storageStub.requested)
}
