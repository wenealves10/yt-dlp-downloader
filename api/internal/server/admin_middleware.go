package server

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
)

// authorizationUserKey guarda o usuário completo já carregado pelo
// authMiddleware, evitando uma segunda ida ao banco na checagem de papel.
const authorizationUserKey = "authorization_user"

// superAdminMiddleware libera a rota apenas para o super admin. Precisa vir
// sempre depois do authMiddleware, que é quem valida o token e carrega o
// usuário.
func superAdminMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		user, ok := currentUser(ctx)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("usuário não autenticado")))
			return
		}

		if user.Role != db.CoreUserRoleSuperAdmin {
			// A mensagem é deliberadamente genérica: um usuário comum não deve
			// nem descobrir que o painel existe.
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(errors.New("acesso negado")))
			return
		}

		ctx.Next()
	}
}

// currentUser devolve o usuário autenticado da requisição.
func currentUser(ctx *gin.Context) (db.User, bool) {
	value, exists := ctx.Get(authorizationUserKey)
	if !exists {
		return db.User{}, false
	}

	user, ok := value.(db.User)
	return user, ok
}

// currentPayload devolve o payload do token da requisição.
func currentPayload(ctx *gin.Context) (*tokens.Payload, bool) {
	value, exists := ctx.Get(authorizationPayloadKey)
	if !exists {
		return nil, false
	}

	payload, ok := value.(*tokens.Payload)
	return payload, ok
}

// ensureSuperAdmin promove o e-mail configurado em SUPER_ADMIN_EMAIL. Existe só
// para o bootstrap do primeiro administrador; depois disso o papel vive no
// banco.
func ensureSuperAdmin(ctx context.Context, store db.Store, email string) {
	if email == "" {
		return
	}

	user, err := store.GetUserByEmail(ctx, email)
	if err != nil {
		log.Printf("bootstrap: SUPER_ADMIN_EMAIL configurado mas o usuário ainda não existe")
		return
	}
	if user.Role == db.CoreUserRoleSuperAdmin {
		return
	}

	if _, err := store.SetUserRoleByEmail(ctx, db.SetUserRoleByEmailParams{
		Email: email,
		Role:  db.CoreUserRoleSuperAdmin,
	}); err != nil {
		log.Printf("bootstrap: falha ao promover super admin: %v", err)
		return
	}
	log.Printf("bootstrap: usuário promovido a super admin user_id=%s", user.ID)
}
