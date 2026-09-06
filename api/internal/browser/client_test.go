package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewClientDevolveNilQuandoNaoConfigurado(t *testing.T) {
	require.False(t, NewClient("", "token", time.Second).Configured())
	require.False(t, NewClient("http://browser:9223", "", time.Second).Configured())
	require.True(t, NewClient("http://browser:9223", "token", time.Second).Configured())
}

func TestClientEnviaOTokenCompartilhado(t *testing.T) {
	var recebido string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recebido = r.Header.Get(authHeader)
		w.Write([]byte(`{"account_id":"x","state":"RUNNING"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "segredo", time.Second)
	_, err := client.SessionInfo(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Equal(t, "segredo", recebido)
}

func TestClientTraduzOsCodigosDeErro(t *testing.T) {
	casos := map[int]error{
		http.StatusConflict:        ErrSessionUnavailable,
		http.StatusTooManyRequests: ErrTooManySessions,
	}

	for status, esperado := range casos {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"error":"detalhe do serviço"}`))
		}))

		client := NewClient(server.URL, "segredo", time.Second)
		_, err := client.StartSession(context.Background(), uuid.New(), "youtube")

		require.ErrorIs(t, err, esperado, status)
		server.Close()
	}
}

func TestClientMarcaIndisponibilidadeDeRede(t *testing.T) {
	// Porta fechada: precisa virar ErrServiceUnavailable, e não um erro
	// genérico, porque quem chama decide o status da conta a partir disso.
	client := NewClient("http://127.0.0.1:1", "segredo", 500*time.Millisecond)
	_, err := client.SessionInfo(context.Background(), uuid.New())

	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestCookiesRecusaJarVazio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("   \n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "segredo", time.Second)
	_, err := client.Cookies(context.Background(), uuid.New(), "youtube")

	require.Error(t, err)
}

func TestUserMessageNaoVazaDetalhesInternos(t *testing.T) {
	interno := errors.New(`Get "http://browser:9223/internal/sessions/x/cookies": dial tcp: lookup browser`)
	envolvido := errors.Join(ErrServiceUnavailable, interno)

	mensagem := UserMessage(envolvido)

	require.Equal(t, "o serviço de navegador não respondeu", mensagem)
	require.NotContains(t, mensagem, "9223")
	require.Empty(t, UserMessage(nil))
}
