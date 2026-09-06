package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// contextoCom monta um gin.Context só com a query string, que é tudo que
// resolverPeriodo lê.
func contextoCom(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?"+query, nil)
	return ctx
}

func TestResolverPeriodoAtalhos(t *testing.T) {
	casos := []struct {
		query       string
		granularity string
		duracaoDias float64
	}{
		{"range=today", "hour", 1},
		{"range=yesterday", "hour", 1},
		{"range=7d", "day", 7},
		{"range=15d", "day", 15},
		{"range=30d", "day", 30},
		{"range=90d", "week", 90},
		// O padrão, quando nenhum range é informado.
		{"", "day", 30},
	}

	for _, caso := range casos {
		intervalo, err := resolverPeriodo(contextoCom(caso.query))
		require.NoError(t, err, caso.query)
		require.Equal(t, caso.granularity, intervalo.Granularity, caso.query)

		dias := intervalo.Ate.Sub(intervalo.De).Hours() / 24
		require.InDelta(t, caso.duracaoDias, dias, 0.1, caso.query)
		require.True(t, intervalo.Ate.After(intervalo.De), caso.query)
	}
}

func TestResolverPeriodoOntemTerminaOndeHojeComeca(t *testing.T) {
	hoje, err := resolverPeriodo(contextoCom("range=today"))
	require.NoError(t, err)
	ontem, err := resolverPeriodo(contextoCom("range=yesterday"))
	require.NoError(t, err)

	// Sem essa emenda, um dos dois períodos perderia ou duplicaria uma hora.
	require.Equal(t, hoje.De, ontem.Ate)
}

func TestResolverPeriodoPersonalizadoIncluiODiaFinal(t *testing.T) {
	intervalo, err := resolverPeriodo(contextoCom("from=2026-01-10&to=2026-01-12"))
	require.NoError(t, err)

	// O fim é exclusivo na consulta: para incluir o dia 12 inteiro ele precisa
	// ir até a meia-noite do dia 13.
	require.Equal(t, 3*24*time.Hour, intervalo.Ate.Sub(intervalo.De))
	require.Equal(t, "day", intervalo.Granularity)
}

func TestResolverPeriodoPersonalizadoDeUmDiaSo(t *testing.T) {
	intervalo, err := resolverPeriodo(contextoCom("from=2026-01-10&to=2026-01-10"))
	require.NoError(t, err)

	require.Equal(t, 24*time.Hour, intervalo.Ate.Sub(intervalo.De))
	// Um único dia é melhor lido por hora.
	require.Equal(t, "hour", intervalo.Granularity)
}

func TestResolverPeriodoRecusaEntradasInvalidas(t *testing.T) {
	invalidos := []string{
		"from=10/01/2026&to=2026-01-12", // formato errado
		"from=2026-01-10&to=nao-e-data",
		"from=2026-01-12&to=2026-01-10", // invertido
		"from=2020-01-01&to=2026-01-01", // acima de um ano
	}

	for _, query := range invalidos {
		_, err := resolverPeriodo(contextoCom(query))
		require.Error(t, err, query)
	}
}

func TestGranularidadePara(t *testing.T) {
	// 90 dias em baldes de hora dariam 2160 pontos e um gráfico ilegível.
	require.Equal(t, "hour", granularidadePara(24*time.Hour))
	require.Equal(t, "day", granularidadePara(30*24*time.Hour))
	require.Equal(t, "week", granularidadePara(90*24*time.Hour))
	require.Equal(t, "month", granularidadePara(365*24*time.Hour))
}

func TestPaginacaoNormalizaEntradas(t *testing.T) {
	limit, offset, page, perPage := paginacao(contextoCom("page=3&perPage=20"))
	require.Equal(t, 20, limit)
	require.Equal(t, 40, offset)
	require.Equal(t, 3, page)
	require.Equal(t, 20, perPage)

	// Valores absurdos ou ausentes não podem virar uma varredura de tabela.
	for _, query := range []string{"", "page=0&perPage=0", "page=-5&perPage=-1", "page=abc&perPage=xyz"} {
		limit, offset, page, _ := paginacao(contextoCom(query))
		require.Equal(t, 20, limit, query)
		require.Equal(t, 0, offset, query)
		require.Equal(t, 1, page, query)
	}

	limit, _, _, _ = paginacao(contextoCom("perPage=100000"))
	require.Equal(t, 100, limit, "perPage precisa ter teto")
}
