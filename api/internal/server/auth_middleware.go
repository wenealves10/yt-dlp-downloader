package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
)

const (
	authorizationHeaderKey  = "authorization"
	authorizationTypeBearer = "bearer"
	authorizationPayloadKey = "authorization_payload"
)

func authMiddleware(tokenCreator tokens.TokenCreator, store db.Store) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authorizationHeader := ctx.GetHeader(authorizationHeaderKey)
		if len(authorizationHeader) == 0 {
			err := errors.New("authorization header is not provided")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(err))
			return
		}

		fields := strings.Fields(authorizationHeader)
		if len(fields) != 2 {
			err := errors.New("invalid authorization header format")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(err))
			return
		}

		authorizationType := strings.ToLower(fields[0])
		if authorizationType != authorizationTypeBearer {
			err := fmt.Errorf("unsupported authorization type %s", authorizationType)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(err))
			return
		}

		if !authorizeToken(ctx, tokenCreator, store, fields[1]) {
			return
		}

		ctx.Next()
	}
}

// formAuthMiddleware is used only by the browser-download form. A native form
// submission starts the file transfer immediately, whereas a link cannot send
// the Authorization header required by the API.
func formAuthMiddleware(tokenCreator tokens.TokenCreator, store db.Store) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := ctx.PostForm("access_token")
		if accessToken == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("access token is not provided")))
			return
		}

		if !authorizeToken(ctx, tokenCreator, store, accessToken) {
			return
		}

		ctx.Next()
	}
}

func authorizeToken(ctx *gin.Context, tokenCreator tokens.TokenCreator, store db.Store, accessToken string) bool {
	payload, err := tokenCreator.VerifyToken(accessToken)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(err))
		return false
	}

	// Check if the user exists in the database
	user, err := store.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("usuário não encontrado")))
		return false
	}

	if !user.Active {
		ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(errors.New("conta desativada")))
		return false
	}

	if !user.IsVerified {
		ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(errors.New("e-mail não verificado")))
		return false
	}

	ctx.Set(authorizationPayloadKey, payload)
	return true
}
