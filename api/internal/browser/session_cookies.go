package browser

import "strings"

// IsSessionCookie informa se o cookie faz parte da sessão autenticada da
// plataforma.
//
// A lista é o contrato entre quem lê o perfil (o serviço de navegador) e quem
// consome a sessão (o downloader): os dois precisam concordar sobre o que
// significa "esta conta está autenticada". Ela é por plataforma porque o mesmo
// nome tem donos diferentes — `sessionid` é do Instagram e do TikTok, e um não
// prova nada sobre o outro.
func IsSessionCookie(plataforma, name string) bool {
	for _, esperado := range PerfilDe(plataforma).CookiesDeSessao {
		if esperado == name {
			return true
		}
	}
	return false
}

// CountSessionCookies conta os cookies de sessão em um jar no formato Netscape.
// Serve para descartar uma conta obviamente deslogada sem gastar uma requisição
// à plataforma. Nenhum valor de cookie é lido, apenas os nomes.
func CountSessionCookies(plataforma string, jar []byte) int {
	found := 0

	for _, line := range strings.Split(string(jar), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Formato Netscape: domínio, subdomínios, caminho, seguro, expiração,
		// nome, valor.
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		if IsSessionCookie(plataforma, fields[5]) {
			found++
		}
	}

	return found
}
