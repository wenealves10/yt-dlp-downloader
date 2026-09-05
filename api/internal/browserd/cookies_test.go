package browserd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatNetscape(t *testing.T) {
	jar := formatNetscape([]cdpCookie{
		{Name: "SID", Value: "abc", Domain: ".youtube.com", Path: "/", Secure: true, Expires: 1767225600},
		{Name: "VISITOR", Value: "xyz", Domain: "www.youtube.com", Path: "/embed", Session: true},
	})

	lines := strings.Split(strings.TrimSpace(string(jar)), "\n")
	require.Equal(t, "# Netscape HTTP Cookie File", lines[0])

	// Cookie de domínio: o ponto inicial vira a flag de subdomínios.
	require.Equal(t, ".youtube.com\tTRUE\t/\tTRUE\t1767225600\tSID\tabc", lines[len(lines)-2])

	// Cookie de sessão não tem expiração: o yt-dlp espera 0.
	require.Equal(t, "www.youtube.com\tFALSE\t/embed\tFALSE\t0\tVISITOR\txyz", lines[len(lines)-1])
}

func TestFormatNetscapeUsaBarraQuandoOPathEstaVazio(t *testing.T) {
	jar := string(formatNetscape([]cdpCookie{{Name: "A", Value: "1", Domain: "youtube.com"}}))
	require.Contains(t, jar, "youtube.com\tFALSE\t/\tFALSE\t0\tA\t1")
}

func TestMatchesCookieDomain(t *testing.T) {
	permitidos := []string{
		"youtube.com", ".youtube.com", "www.youtube.com",
		"accounts.google.com", ".google.com", "r1---sn-x.googlevideo.com", "i.ytimg.com",
	}
	for _, domain := range permitidos {
		require.True(t, matchesCookieDomain(domain), domain)
	}

	// Domínios que apenas terminam parecido não podem passar.
	recusados := []string{
		"evil.com", "notgoogle.com", "youtube.com.evil.com",
		"googlevideo.com.attacker.net", "", "fakeyoutube.com",
	}
	for _, domain := range recusados {
		require.False(t, matchesCookieDomain(domain), domain)
	}
}

func TestFilterCookiesDescartaOutrosSites(t *testing.T) {
	filtrados := filterCookies([]cdpCookie{
		{Name: "A", Domain: ".youtube.com"},
		{Name: "B", Domain: "banco.example.com"},
		{Name: "C", Domain: ".google.com"},
	})

	require.Len(t, filtrados, 2)
	for _, cookie := range filtrados {
		require.NotEqual(t, "banco.example.com", cookie.Domain)
	}
}
