package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// A partir daqui existem DUAS formas de provar identidade nesta API:
//
//   - um token de acesso, emitido no login, que representa uma PESSOA;
//   - uma chave de API, emitida no painel, que representa um SISTEMA.
//
// Os dois middlewares terminam no mesmo lugar: com a linha de `users`
// autenticada guardada no contexto. As funções abaixo são a única coisa que os
// handlers de download precisam saber sobre isso — e é por isso que as mesmas
// rotas atendem os dois casos sem um `if` espalhado por elas.
//
// Antes disto, cada handler lia o payload do token com
// `ctx.MustGet(...).(*tokens.Payload)` e ia ao banco buscar o usuário de novo.
// Além de repetir consulta já feita pelo middleware, um MustGet em rota sem
// autenticação derruba o processo em vez de responder 401.

// usuarioAtual devolve quem está fazendo a requisição, pessoa ou sistema.
func usuarioAtual(ctx *gin.Context) (db.User, bool) {
	return currentUser(ctx)
}

// usuarioAtualID devolve o id de quem faz a requisição e já responde 401
// quando não há ninguém autenticado.
//
// O erro é respondido aqui, e não devolvido para quem chama, porque todos os
// chamadores fariam exatamente a mesma coisa com ele — e um deles esqueceria.
func usuarioAtualID(ctx *gin.Context) (uuid.UUID, bool) {
	user, ok := currentUser(ctx)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized,
			errorResponse(errors.New("requisição não autenticada")))
		return uuid.Nil, false
	}
	return user.ID, true
}
