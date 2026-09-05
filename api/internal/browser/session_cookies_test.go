package browser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCountSessionCookies(t *testing.T) {
	jar := []byte("# Netscape HTTP Cookie File\n\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1767225600\tSID\tvalor\n" +
		".google.com\tTRUE\t/\tTRUE\t1767225600\t__Secure-3PSID\tvalor\n" +
		".youtube.com\tTRUE\t/\tFALSE\t0\tVISITOR_INFO1_LIVE\tvalor\n")

	// Só os cookies de sessão contam; VISITOR_INFO1_LIVE existe mesmo deslogado.
	require.Equal(t, 2, CountSessionCookies(jar))
}

func TestCountSessionCookiesEmPerfilDeslogado(t *testing.T) {
	jar := []byte("# Netscape HTTP Cookie File\n\n" +
		".google.com\tTRUE\t/\tTRUE\t1767225600\tNID\tvalor\n" +
		"accounts.google.com\tFALSE\t/\tTRUE\t0\tOTZ\tvalor\n")

	require.Zero(t, CountSessionCookies(jar))
}

func TestCountSessionCookiesIgnoraLinhasMalFormadas(t *testing.T) {
	jar := []byte("# comentário\n\nlinha solta\n.youtube.com\tTRUE\t/\n")
	require.Zero(t, CountSessionCookies(jar))
}
