package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Códigos de erro da API de integrações.
//
// Existem porque a MENSAGEM não é contrato: ela está em português, pode ser
// reescrita para ficar mais clara e um dia pode ser traduzida. O código é o que
// o sistema cliente usa no `switch` — "se der quota_exceeded, espera até amanhã;
// se der rate_limited, respeita o Retry-After; se der content_unavailable,
// marca o item como inválido e não tenta mais".
//
// Toda resposta de erro desta API carrega um destes valores. Um erro sem código
// obrigaria o cliente a comparar texto, que é o tipo de acoplamento que quebra
// silenciosamente no dia em que alguém corrige uma vírgula.
const (
	// Autenticação e autorização.
	CodeMissingCredentials  = "missing_credentials"
	CodeInvalidAPIKey       = "invalid_api_key"
	CodeKeyRevoked          = "key_revoked"
	CodeKeyExpired          = "key_expired"
	CodeIntegrationMismatch = "integration_mismatch"
	CodeIntegrationDisabled = "integration_disabled"
	CodeIPNotAllowed        = "ip_not_allowed"

	// Contenção de abuso.
	CodeRateLimited       = "rate_limited"
	CodeQuotaExceeded     = "quota_exceeded"
	CodeTooManyConcurrent = "too_many_concurrent"
	CodeFileTooLarge      = "file_too_large"

	// Requisição.
	CodeInvalidRequest     = "invalid_request"
	CodeNotFound           = "not_found"
	CodeMethodNotAllowed   = "method_not_allowed"
	CodeDownloadNotReady   = "download_not_ready"
	CodeDownloadExpired    = "download_expired"
	CodeWebhookNotTested   = "webhook_test_failed"
	CodeInternalError      = "internal_error"
	CodeServiceUnavailable = "service_unavailable"
)

// chaveCodigoErro guarda no contexto o código já respondido, para a auditoria
// registrá-lo sem precisar reabrir o corpo da resposta.
const chaveCodigoErro = "integration_error_code"

// erroIntegracao responde um erro da API de integrações.
//
// É o único caminho de erro desta superfície, e é isso que garante que o par
// (status, código) sempre saia junto. O `code` no corpo repete a intenção do
// status HTTP em um vocabulário mais fino: 429 pode ser "muitas chamadas por
// minuto" ou "cota do dia esgotada", e são reações completamente diferentes do
// lado do cliente.
func erroIntegracao(ctx *gin.Context, status int, codigo, mensagem string) {
	ctx.Set(chaveCodigoErro, codigo)
	ctx.AbortWithStatusJSON(status, gin.H{
		"error": mensagem,
		"code":  codigo,
	})
}

// codigoRespondido devolve o código registrado nesta requisição, se houver.
func codigoRespondido(ctx *gin.Context) string {
	if valor, existe := ctx.Get(chaveCodigoErro); existe {
		if codigo, ok := valor.(string); ok {
			return codigo
		}
	}
	return ""
}

// registrarCodigoErro anota o código de um erro respondido por um handler
// compartilhado, que monta o corpo por conta própria.
//
// Os handlers de download são usados pelas duas superfícies (a do usuário na
// tela e a da integração), e o corpo deles já leva `code`. Esta função só
// espelha esse valor no contexto para que a auditoria o veja.
func registrarCodigoErro(ctx *gin.Context, codigo string) {
	if codigo != "" {
		ctx.Set(chaveCodigoErro, codigo)
	}
}

// codigoPadraoDoStatus é a rede de segurança da auditoria: se um caminho de
// erro escapar sem código anotado, o registro ainda diz que classe de problema
// foi, em vez de deixar a linha em branco.
func codigoPadraoDoStatus(status int) string {
	switch {
	case status == http.StatusNotFound:
		return CodeNotFound
	case status == http.StatusMethodNotAllowed:
		return CodeMethodNotAllowed
	case status == http.StatusUnauthorized:
		return CodeInvalidAPIKey
	case status == http.StatusTooManyRequests:
		return CodeRateLimited
	case status == http.StatusConflict:
		return CodeDownloadNotReady
	case status == http.StatusGone:
		return CodeDownloadExpired
	case status >= 500:
		return CodeInternalError
	case status >= 400:
		return CodeInvalidRequest
	}
	return ""
}
