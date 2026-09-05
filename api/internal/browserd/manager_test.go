package browserd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
