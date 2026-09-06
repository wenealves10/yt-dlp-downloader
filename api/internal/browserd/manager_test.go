package browserd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{cfg: Config{ProfilesDir: t.TempDir()}}
}

func TestProfilePathAceitaApenasUUID(t *testing.T) {
	manager := newTestManager(t)

	path, err := manager.ProfilePath("11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)

	root, err := filepath.Abs(manager.cfg.ProfilesDir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "11111111-1111-4111-8111-111111111111"), path)
}

func TestProfilePathRecusaEntradasPerigosas(t *testing.T) {
	manager := newTestManager(t)

	// Qualquer coisa que não seja UUID é recusada, o que fecha path traversal,
	// caminhos absolutos e nomes de arquivo arbitrários.
	entradas := []string{
		"", "..", "../..", "../../etc/passwd", "/etc/passwd",
		"conta", "11111111-1111-4111-8111-111111111111/../outra",
		"00000000-0000-0000-0000-000000000000",
	}
	for _, entrada := range entradas {
		_, err := manager.ProfilePath(entrada)
		require.ErrorIs(t, err, ErrInvalidAccountID, entrada)
	}
}

func TestProfilePathNormalizaMaiusculas(t *testing.T) {
	manager := newTestManager(t)

	// O UUID é normalizado, então a mesma conta nunca gera dois perfis.
	maiusculo, err := manager.ProfilePath("AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE")
	require.NoError(t, err)
	minusculo, err := manager.ProfilePath("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	require.NoError(t, err)
	require.Equal(t, minusculo, maiusculo)
}

func TestRandomVNCPassword(t *testing.T) {
	// O protocolo RFB só considera os 8 primeiros bytes da senha.
	primeira, err := randomVNCPassword()
	require.NoError(t, err)
	require.Len(t, primeira, vncPasswordLength)

	segunda, err := randomVNCPassword()
	require.NoError(t, err)
	require.NotEqual(t, primeira, segunda)
}

func TestChromeArgsSandboxLigado(t *testing.T) {
	manager := &Manager{cfg: Config{Width: 1440, Height: 900}}

	args := manager.chromeArgs("/data/profiles/abc", 9222, browser.PerfilDe("youtube").LoginURL)

	// Com o sandbox ligado não há motivo para desligá-lo nem para silenciar um
	// aviso que o Chrome não vai emitir.
	require.NotContains(t, args, "--no-sandbox")
	require.NotContains(t, args, "--test-type")
	require.Contains(t, args, "--user-data-dir=/data/profiles/abc")
	// Nenhuma flag de automação: elas são o que faz o login do Google recusar.
	require.NotContains(t, args, "--enable-automation")
	require.NotContains(t, args, "--headless")
}

func TestChromeArgsSandboxDesligadoSilenciaOAviso(t *testing.T) {
	manager := &Manager{cfg: Config{Width: 1440, Height: 900, DisableSandbox: true}}

	args := manager.chromeArgs("/data/profiles/abc", 9222, browser.PerfilDe("youtube").LoginURL)

	require.Contains(t, args, "--no-sandbox")
	// Sem --test-type, o Chrome desenha uma faixa amarela no topo de toda
	// janela; ela rouba altura da tela remota e alarma quem só quer fazer
	// login. As duas andam juntas.
	require.Contains(t, args, "--test-type")
	require.NotContains(t, args, "--enable-automation")
}

func TestChromeArgsUsaAUrlDeLoginPorUltimo(t *testing.T) {
	manager := &Manager{cfg: Config{Width: 1440, Height: 900}}

	args := manager.chromeArgs("/data/profiles/abc", 9222, browser.PerfilDe("youtube").LoginURL)

	// A URL fecha a lista: qualquer coisa depois dela seria interpretada como
	// mais um argumento do processo.
	require.Equal(t, browser.PerfilDe("youtube").LoginURL, args[len(args)-1])
}

// Cada plataforma abre na SUA tela de login. Abrir sempre no Google deixaria
// quem cadastrou uma conta do Vimeo olhando para o formulário errado.
func TestChromeArgsAbreNoLoginDaPlataforma(t *testing.T) {
	manager := &Manager{cfg: Config{Width: 1440, Height: 900}}

	for _, perfil := range browser.PlataformasSuportadas() {
		args := manager.chromeArgs("/data/profiles/abc", 9222, perfil.LoginURL)
		require.Equal(t, perfil.LoginURL, args[len(args)-1], perfil.ID)
	}
}
