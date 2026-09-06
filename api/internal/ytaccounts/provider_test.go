package ytaccounts

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
)

func TestAuthArgsUsaASessaoGerenciada(t *testing.T) {
	configs.LoadedConfig = configs.Config{
		YoutubeDLFileCookies: "/etc/cookies-estaticos.txt",
		YoutubeDLUserAgent:   "UA",
		YoutubeDLReferer:     "https://www.youtube.com/",
	}

	args := AuthArgs(&Lease{CookieFile: "/tmp/sessao/cookies.txt"})

	require.Contains(t, args, "/tmp/sessao/cookies.txt")
	require.NotContains(t, args, "/etc/cookies-estaticos.txt")
	require.Contains(t, args, "--user-agent")
	require.Contains(t, args, "--referer")
}

func TestAuthArgsSemContaUsaOArquivoEstatico(t *testing.T) {
	configs.LoadedConfig = configs.Config{YoutubeDLFileCookies: "/etc/cookies-estaticos.txt"}

	args := AuthArgs(nil)
	require.Contains(t, args, "/etc/cookies-estaticos.txt")
}

func TestAuthArgsCombinaProxyComCookies(t *testing.T) {
	// Antes, ligar o proxy fazia o yt-dlp rodar sem nenhum cookie. As duas
	// coisas são independentes e precisam conviver.
	configs.LoadedConfig = configs.Config{
		ProxyEnabled: true,
		ProxyURL:     "http://proxy.interno:7000",
	}

	args := AuthArgs(&Lease{CookieFile: "/tmp/sessao/cookies.txt"})

	require.Contains(t, args, "--proxy")
	require.Contains(t, args, "http://proxy.interno:7000")
	require.Contains(t, args, "--cookies")
	require.Contains(t, args, "/tmp/sessao/cookies.txt")
}

func TestAuthArgsSemCookiesQuandoNadaEstaConfigurado(t *testing.T) {
	configs.LoadedConfig = configs.Config{}

	args := AuthArgs(nil)
	require.NotContains(t, args, "--cookies")
	require.Contains(t, args, "--add-header")
}

func TestLeaseReleaseApagaOArquivoTemporario(t *testing.T) {
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "cookies.txt")
	require.NoError(t, os.WriteFile(cookieFile, []byte("dados"), 0o600))

	lease := &Lease{AccountID: uuid.New(), CookieFile: cookieFile, tempDir: tempDir}
	lease.Release()

	_, err := os.Stat(tempDir)
	require.True(t, os.IsNotExist(err))

	// Release é idempotente: os jobs chamam sempre via defer.
	require.NotPanics(t, lease.Release)
}

func TestReleaseEmLeaseNuloNaoQuebra(t *testing.T) {
	// Acquire devolve nil quando não há conta; o defer no job continua válido.
	var lease *Lease
	require.NotPanics(t, lease.Release)
}

func TestIsAuthFailureReconheceSessaoRecusada(t *testing.T) {
	recusas := [][]byte{
		[]byte("ERROR: [youtube] abc: The provided YouTube account cookies are no longer valid. They have likely been rotated."),
		[]byte("ERROR: [youtube] abc: Sign in to confirm you're not a bot. Use --cookies-from-browser or --cookies for the authentication."),
		[]byte("ERROR: [youtube] abc: Sign in to confirm your age. This video may be inappropriate for some users."),
	}
	for _, saida := range recusas {
		require.True(t, IsAuthFailure(saida), string(saida))
		require.Contains(t, AuthFailureReason(saida), "a plataforma recusou a sessão")
	}
}

func TestIsAuthFailureIgnoraProblemasQueNaoSaoDeSessao(t *testing.T) {
	// Nenhuma destas deve tirar uma conta boa do rodízio.
	outras := [][]byte{
		// Falta de runtime JavaScript, não sessão expirada.
		[]byte("ERROR: [youtube] abc: The page needs to be reloaded."),
		[]byte("ERROR: [youtube] abc: Video unavailable. This video is private."),
		[]byte("ERROR: [youtube] abc: This video is available to this channel's members"),
		[]byte("ERROR: unable to download video data: HTTP Error 403: Forbidden"),
		[]byte("WARNING: [youtube] n challenge solving failed"),
		nil,
		[]byte(""),
	}
	for _, saida := range outras {
		require.False(t, IsAuthFailure(saida), string(saida))
		require.Empty(t, AuthFailureReason(saida))
	}
}

func TestReportAuthFailureIgnoraChamadaSemConta(t *testing.T) {
	// Sem provider ou sem lease, o download seguiu sem conta gerenciada e não
	// há nada a marcar.
	require.NotPanics(t, func() {
		ReportAuthFailure(context.Background(), nil, nil, []byte("Please sign in"))
		ReportAuthFailure(context.Background(), (*Manager)(nil), nil, []byte("Please sign in"))
	})
}

// As outras plataformas recusam a sessão com outras palavras. Sem reconhecê-las,
// um cookie expirado do Vimeo ficaria no rodízio para sempre, derrubando todo
// download que o pegasse.
func TestIsAuthFailureReconheceOutrasPlataformas(t *testing.T) {
	casos := map[string]string{
		"vimeo":  "ERROR: [vimeo] 123: The web client only works when logged-in.",
		"reddit": "ERROR: [reddit] abc: Login required to view this content",
		"x":      "ERROR: [twitter] 1: You must be logged in to view this Tweet.",
	}

	for nome, saida := range casos {
		if !IsAuthFailure([]byte(saida)) {
			t.Errorf("[%s] deveria ser reconhecido como sessão recusada: %q", nome, saida)
		}
	}
}

// Restrição de conteúdo não é sessão expirada: tirar a conta do rodízio por
// causa de um vídeo privado gastaria todas as contas boas do administrador.
func TestIsAuthFailureIgnoraRestricaoDeConteudo(t *testing.T) {
	casos := []string{
		"ERROR: [youtube] abc: Private video. Sign in if you've been granted access to this video",
		"ERROR: [youtube] abc: Video unavailable. This video is not available in your country",
		"ERROR: [vimeo] 1: The page needs to be reloaded",
	}

	for _, saida := range casos {
		if IsAuthFailure([]byte(saida)) {
			t.Errorf("não deveria derrubar a conta: %q", saida)
		}
	}
}

// "This content isn't available" é ambíguo por natureza: o Facebook responde
// isso tanto para post apagado quanto para sessão expirada. Tratá-lo como falha
// de autenticação derrubaria contas boas por causa de um conteúdo removido.
func TestIsAuthFailureIgnoraMensagemAmbigua(t *testing.T) {
	if IsAuthFailure([]byte("ERROR: [facebook] 1: This content isn't available right now")) {
		t.Error("mensagem ambígua não pode tirar a conta do rodízio")
	}
}
