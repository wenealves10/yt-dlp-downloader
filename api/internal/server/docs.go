package server

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// A especificação é EMBUTIDA no binário, e isso é o ponto.
//
// Uma especificação em arquivo solto no repositório envelhece sem ninguém
// notar: ela não quebra nenhum teste e não impede nenhum deploy. Embutida e
// publicada pelo próprio serviço, ela é versionada junto com o código, sobe no
// mesmo container e responde na mesma URL — se a rota mudou e a especificação
// não, os dois estão visivelmente no mesmo lugar para quem for conferir.
//
//go:embed assets/openapi.yaml
var especificacaoOpenAPI []byte

//go:embed assets/docs.html
var paginaDocumentacao []byte

// registrarDocumentacao publica a documentação da API.
//
// É intencionalmente PÚBLICA (sem autenticação): a documentação não contém
// segredo nenhum — descreve rotas, campos e códigos de erro — e exigir
// credencial para lê-la significaria que quem vai integrar precisa da chave
// antes de conseguir avaliar se dá para integrar. Quem quiser fechar isso
// desliga com DOCS_ENABLED=false.
func (s *Server) registrarDocumentacao(router *gin.Engine) {
	router.GET("/openapi.yaml", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "public, max-age=300")
		ctx.Data(http.StatusOK, "application/yaml; charset=utf-8", especificacaoOpenAPI)
	})

	// Alias em .json não existe de propósito: converter YAML para JSON em cada
	// requisição gastaria CPU para entregar o mesmo documento em outra sintaxe,
	// que qualquer ferramenta de OpenAPI já lê.
	router.GET("/docs", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "public, max-age=300")
		ctx.Data(http.StatusOK, "text/html; charset=utf-8", paginaDocumentacao)
	})
}
