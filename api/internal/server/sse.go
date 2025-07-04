package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) sseHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := ctx.Query("token")
		payload, err := s.tokenCreator.VerifyToken(accessToken)
		if err != nil {
			ctx.String(http.StatusUnauthorized, "Token inválido: %v", err)
			return
		}

		user, err := s.store.GetUserByEmail(ctx, payload.Email)
		if err != nil {
			ctx.String(http.StatusUnauthorized, "Usuário não encontrado")
			return
		}
		if !user.Active {
			ctx.String(http.StatusForbidden, "Conta desativada")
			return
		}
		if !user.IsVerified {
			ctx.String(http.StatusForbidden, "E-mail não verificado")
			return
		}

		ctx.Header("Content-Type", "text/event-stream")
		ctx.Header("Cache-Control", "no-cache")
		ctx.Header("Connection", "keep-alive")

		flusher, ok := ctx.Writer.(http.Flusher)
		if !ok {
			ctx.String(http.StatusInternalServerError, "Streaming não suportado")
			return
		}

		sub := s.sseManager.Subscribe(payload.UserID)
		defer s.sseManager.Unsubscribe(payload.UserID, sub)

		for {
			select {
			case <-ctx.Request.Context().Done():
				return
			case msg := <-sub:
				fmt.Fprintf(ctx.Writer, "data: %s\n\n", msg)
				flusher.Flush()
			}
		}
	}
}
