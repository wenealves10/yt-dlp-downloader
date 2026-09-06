package server

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

func contextoComUsuario(user db.User, autenticado bool) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if autenticado {
		ctx.Set(authorizationUserKey, user)
	}
	return ctx
}

// O detalhe técnico é o que responde "por que falhou?" sem entrar no container.
// Sem ele, um 403 da plataforma e um provider quebrado viram a mesma frase.
func TestRespostaDeErroMostraDetalheAoSuperAdmin(t *testing.T) {
	erro := &media.Error{
		Kind:   media.ErrBlocked,
		Detail: "ERROR: [Reddit] abc: HTTP Error 403: Forbidden",
	}

	corpo := respostaDeErro(
		contextoComUsuario(db.User{Role: db.CoreUserRoleSuperAdmin}, true), erro)

	if corpo["detail"] != erro.Detail {
		t.Fatalf("o super admin deveria ver o detalhe, obtido %v", corpo["detail"])
	}
	if corpo["code"] != "blocked" {
		t.Errorf("código esperado \"blocked\", obtido %v", corpo["code"])
	}
	if corpo["error"] != media.ErrBlocked.Error() {
		t.Errorf("mensagem inesperada: %v", corpo["error"])
	}
}

// Para o usuário comum o detalhe não significa nada e expõe interno à toa.
func TestRespostaDeErroEscondeDetalheDoUsuarioComum(t *testing.T) {
	erro := &media.Error{
		Kind:   media.ErrBlocked,
		Detail: "ERROR: [Reddit] abc: HTTP Error 403: Forbidden",
	}

	for _, papel := range []db.CoreUserRole{db.CoreUserRoleUser, db.CoreUserRoleAdmin} {
		corpo := respostaDeErro(contextoComUsuario(db.User{Role: papel}, true), erro)

		if _, existe := corpo["detail"]; existe {
			t.Errorf("[%s] o detalhe técnico não pode sair para este papel", papel)
		}
		// A mensagem de domínio diz "a partir deste servidor". Isso é nosso.
		if corpo["error"] != media.PublicMessage(erro) {
			t.Errorf("[%s] deveria receber a mensagem pública, obteve %v", papel, corpo["error"])
		}
		if corpo["code"] != "unavailable" {
			t.Errorf("[%s] o código deveria ser colapsado, obteve %v", papel, corpo["code"])
		}
	}
}

// A regra dura, verificada no corpo inteiro da resposta: nada que descreva a
// nossa infraestrutura pode sair para quem não é super admin.
func TestRespostaDeErroNaoVazaInternoParaNaoAdmin(t *testing.T) {
	erros := []*media.Error{
		{Kind: media.ErrBlocked, Detail: "HTTP Error 403: Blocked"},
		{Kind: media.ErrNetwork, Detail: "connection refused"},
		{Kind: media.ErrProviderUnavailable, Detail: "exec: yt-dlp not found"},
		{Kind: media.ErrDownloadFailed, Detail: "ERROR: [Reddit] cookies inválidos"},
	}
	proibidos := []string{"servidor", "provider", "sess", "cookie", "yt-dlp", "403", "reddit"}

	for _, erro := range erros {
		for _, papel := range []db.CoreUserRole{db.CoreUserRoleUser, db.CoreUserRoleAdmin} {
			corpo := respostaDeErro(contextoComUsuario(db.User{Role: papel}, true), erro)

			var junto string
			for _, valor := range corpo {
				junto += " " + strings.ToLower(fmt.Sprint(valor))
			}
			for _, termo := range proibidos {
				if strings.Contains(junto, termo) {
					t.Errorf("[%s/%v] a resposta vazou %q: %s", papel, erro.Kind, termo, junto)
				}
			}
		}
	}
}

// E o super admin continua vendo tudo — sem isso, diagnosticar exige entrar no
// container.
func TestSuperAdminContinuaVendoOMotivoPreciso(t *testing.T) {
	erro := &media.Error{Kind: media.ErrBlocked, Detail: "HTTP Error 403: Blocked"}

	corpo := respostaDeErro(contextoComUsuario(db.User{Role: db.CoreUserRoleSuperAdmin}, true), erro)

	if corpo["error"] != media.UserMessage(erro) {
		t.Errorf("o super admin deveria ver a mensagem precisa, obteve %v", corpo["error"])
	}
	if corpo["code"] != "blocked" {
		t.Errorf("o código não deveria ser colapsado para o super admin, obteve %v", corpo["code"])
	}
	if corpo["detail"] != erro.Detail {
		t.Errorf("o detalhe deveria estar presente, obteve %v", corpo["detail"])
	}
}

// Requisição sem usuário carregado não pode virar vazamento por descuido.
func TestRespostaDeErroSemUsuarioNaoVazaDetalhe(t *testing.T) {
	erro := &media.Error{Kind: media.ErrBlocked, Detail: "interno"}

	corpo := respostaDeErro(contextoComUsuario(db.User{}, false), erro)

	if _, existe := corpo["detail"]; existe {
		t.Error("sem usuário identificado, nada de detalhe")
	}
}

// Erro sem detalhe não inventa campo vazio na resposta.
func TestRespostaDeErroSemDetalheOmiteOCampo(t *testing.T) {
	corpo := respostaDeErro(
		contextoComUsuario(db.User{Role: db.CoreUserRoleSuperAdmin}, true),
		&media.Error{Kind: media.ErrContentPrivate})

	if _, existe := corpo["detail"]; existe {
		t.Error("sem detalhe, o campo não deveria aparecer")
	}
}

// A descrição da sessão é a primeira pergunta quando falha numa plataforma que
// exige login: a conta cadastrada chegou a ser usada?
func TestDescricaoDaSessao(t *testing.T) {
	casos := []struct {
		nome     string
		sessao   sessaoUsada
		esperado string
	}{
		{
			"plataforma que não precisa de conta",
			sessaoUsada{Plataforma: "youtube"},
			"esta plataforma resolve sem conta",
		},
		{
			"conta emprestada",
			sessaoUsada{Plataforma: "reddit", Necessaria: true, Conta: "minha conta"},
			`usando a conta "minha conta"`,
		},
		{
			"precisa e não tem",
			sessaoUsada{
				Plataforma: "reddit", Necessaria: true,
				Motivo: "nenhuma conta autenticada de Reddit disponível",
			},
			"nenhuma conta autenticada de Reddit disponível",
		},
	}

	for _, caso := range casos {
		if obtido := caso.sessao.Descricao(); obtido != caso.esperado {
			t.Errorf("[%s] esperado %q, obtido %q", caso.nome, caso.esperado, obtido)
		}
	}
}
