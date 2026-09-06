package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateReadablePassword(t *testing.T) {
	senha, err := GenerateReadablePassword()
	require.NoError(t, err)
	require.Len(t, senha, senhaTamanho)

	// Caracteres ambíguos atrapalham quem digita a senha lida na tela.
	for _, proibido := range []string{"0", "O", "1", "l", "I"} {
		require.NotContains(t, senha, proibido)
	}
}

func TestGenerateReadablePasswordNaoRepete(t *testing.T) {
	vistas := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		senha, err := GenerateReadablePassword()
		require.NoError(t, err)
		require.False(t, vistas[senha], "senha repetida: %s", senha)
		vistas[senha] = true
	}
}

func TestSenhaGeradaFuncionaNoLogin(t *testing.T) {
	// A senha entregue ao administrador precisa autenticar de fato.
	senha, err := GenerateReadablePassword()
	require.NoError(t, err)

	hash, err := HashPassword(senha)
	require.NoError(t, err)
	require.NoError(t, CheckPassword(senha, hash))
	require.Error(t, CheckPassword(strings.ToUpper(senha)+"x", hash))
}
