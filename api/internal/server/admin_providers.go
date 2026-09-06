package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// listProviders diagnostica o mecanismo de download. É o que responde
// "por que nenhum download está funcionando?" sem entrar no container.
func (s *Server) listProviders(ctx *gin.Context) {
	if s.mediaRegistry == nil {
		ctx.JSON(http.StatusOK, gin.H{
			"providers": []any{},
			"healthy":   false,
			"detail":    "nenhum provider registrado neste ambiente",
		})
		return
	}

	requestCtx, cancelar := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
	defer cancelar()

	saude := s.mediaRegistry.Health(requestCtx)

	saudavel := len(saude) > 0
	for _, item := range saude {
		if !item.Available {
			saudavel = false
		}
	}

	plataformas := make([]gin.H, 0, len(media.KnownPlatforms()))
	for _, plataforma := range media.KnownPlatforms() {
		plataformas = append(plataformas, gin.H{
			"id":    string(plataforma),
			"label": plataforma.Label(),
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"providers": saude,
		"healthy":   saudavel,
		// A lista do que o yt-dlp entende passa de mil sites e muda a cada
		// versão dele; estas são as plataformas com rótulo próprio na tela.
		"platforms": plataformas,
	})
}
