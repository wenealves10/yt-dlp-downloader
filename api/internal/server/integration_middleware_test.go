package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
)

// contextoComHeaders monta um contexto de requisição com os headers informados.
func contextoComHeaders(headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/integration/me", nil)
	for nome, valor := range headers {
		ctx.Request.Header.Set(nome, valor)
	}
	return ctx
}

// A extração da credencial é a porta de entrada da API de integrações. Aceitar
// de menos quebra clientes legítimos; aceitar de mais (um token de usuário, por
// exemplo) levaria uma credencial do tipo errado até a consulta no banco.
func TestExtracaoDaChaveDeAPI(t *testing.T) {
	chave, err := integrations.GenerateKey()
	if err != nil {
		t.Fatalf("geração falhou: %v", err)
	}

	casos := []struct {
		nome     string
		headers  map[string]string
		esperado string
	}{
		{
			nome:     "bearer, o caminho normal",
			headers:  map[string]string{"Authorization": "Bearer " + chave.Plaintext},
			esperado: chave.Plaintext,
		},
		{
			// Alguns proxies e gateways reescrevem ou removem Authorization.
			nome:     "header alternativo",
			headers:  map[string]string{"X-API-Key": chave.Plaintext},
			esperado: chave.Plaintext,
		},
		{
			// O header alternativo ganha quando os dois vêm: ele é explícito.
			nome: "alternativo tem precedência",
			headers: map[string]string{
				"X-API-Key":     chave.Plaintext,
				"Authorization": "Bearer outra-coisa",
			},
			esperado: chave.Plaintext,
		},
		{
			nome:     "bearer em caixa diferente",
			headers:  map[string]string{"Authorization": "bearer " + chave.Plaintext},
			esperado: chave.Plaintext,
		},
		{
			// Erro comum de integração: colar a chave sem o "Bearer". Ela é
			// reconhecível pelo prefixo, e recusar não protegeria nada.
			nome:     "chave colada sem o esquema",
			headers:  map[string]string{"Authorization": chave.Plaintext},
			esperado: chave.Plaintext,
		},
		{
			nome:     "sem credencial",
			headers:  map[string]string{},
			esperado: "",
		},
		{
			// Um token de usuário sem "Bearer" não tem a forma de uma chave, e
			// por isso não é aceito como uma.
			nome:     "token de usuário colado sem o esquema",
			headers:  map[string]string{"Authorization": "v2.local.algumacoisa"},
			esperado: "",
		},
		{
			nome:     "esquema desconhecido",
			headers:  map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
			esperado: "",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := extrairChaveAPI(contextoComHeaders(caso.headers)); obtido != caso.esperado {
				t.Errorf("extrairChaveAPI() = %q, esperava %q", obtido, caso.esperado)
			}
		})
	}
}

// A cota reseta à meia-noite de São Paulo, o mesmo corte usado pela contagem no
// banco. Em UTC, ela viraria às 21h e o cliente veria o limite resetar no meio
// da tarde — discordando do número que a própria API devolve.
func TestResetDaCotaEhMeiaNoiteEmSaoPaulo(t *testing.T) {
	reset := proximoResetCota()

	if !reset.After(time.Now()) {
		t.Error("o próximo reset tem de estar no futuro")
	}
	if reset.Sub(time.Now()) > 24*time.Hour {
		t.Error("o próximo reset não pode estar a mais de um dia de distância")
	}

	local, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// Imagem sem tzdata: a função cai para UTC de propósito, e aí não há o
		// que conferir sobre o fuso.
		t.Skip("tzdata indisponível neste ambiente")
	}

	emSaoPaulo := reset.In(local)
	if emSaoPaulo.Hour() != 0 || emSaoPaulo.Minute() != 0 || emSaoPaulo.Second() != 0 {
		t.Errorf("esperava meia-noite em São Paulo, veio %s", emSaoPaulo.Format(time.RFC3339))
	}
}

// O código do erro é o que a auditoria grava e o que o cliente trata. Um
// caminho de erro que escape sem código não pode deixar a linha em branco.
func TestCodigoPadraoCobreAsClassesDeErro(t *testing.T) {
	casos := map[int]string{
		http.StatusNotFound:            CodeNotFound,
		http.StatusMethodNotAllowed:    CodeMethodNotAllowed,
		http.StatusUnauthorized:        CodeInvalidAPIKey,
		http.StatusTooManyRequests:     CodeRateLimited,
		http.StatusConflict:            CodeDownloadNotReady,
		http.StatusGone:                CodeDownloadExpired,
		http.StatusInternalServerError: CodeInternalError,
		http.StatusBadGateway:          CodeInternalError,
		http.StatusBadRequest:          CodeInvalidRequest,
		http.StatusUnprocessableEntity: CodeInvalidRequest,
	}

	for status, esperado := range casos {
		if obtido := codigoPadraoDoStatus(status); obtido != esperado {
			t.Errorf("codigoPadraoDoStatus(%d) = %q, esperava %q", status, obtido, esperado)
		}
	}

	// Respostas de sucesso não têm código de erro.
	if codigoPadraoDoStatus(http.StatusOK) != "" {
		t.Error("uma resposta de sucesso não deveria receber código de erro")
	}
}

// O código anotado por um handler compartilhado tem de chegar à auditoria: é
// ele que permite ao painel agrupar falhas por causa em vez de por texto.
func TestCodigoRespondidoViajaNoContexto(t *testing.T) {
	ctx := contextoComHeaders(nil)

	if codigoRespondido(ctx) != "" {
		t.Error("um contexto novo não deveria ter código de erro")
	}

	registrarCodigoErro(ctx, CodeQuotaExceeded)
	if obtido := codigoRespondido(ctx); obtido != CodeQuotaExceeded {
		t.Errorf("codigoRespondido() = %q, esperava %q", obtido, CodeQuotaExceeded)
	}

	// Registrar vazio não pode apagar o que já estava lá.
	registrarCodigoErro(ctx, "")
	if codigoRespondido(ctx) != CodeQuotaExceeded {
		t.Error("registrar um código vazio não deveria sobrescrever o existente")
	}
}
