package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
)

func limitMiddleware(store db.Store) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		payloadAny, exists := ctx.Get(authorizationPayloadKey)
		if !exists {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("token não fornecido")))
			return
		}

		payload, ok := payloadAny.(*tokens.Payload)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("token inválido")))
			return
		}

		// Buscar usuário completo (para acessar o daily_limit)
		user, err := store.GetUserByEmail(ctx, payload.Email)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(errors.New("erro ao carregar usuário")))
			return
		}

		// Contar quantos downloads o usuário já fez hoje
		count, err := store.CountDownloadsToday(ctx, user.ID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(errors.New("erro ao verificar limite de uso")))
			return
		}

		if count >= int64(user.DailyLimit) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "limite diário atingido",
			})
			return
		}

		ctx.Next()
	}
}
