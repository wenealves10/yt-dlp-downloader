package browser

import "strings"

// sessionCookieNames são os cookies que o Google emite para uma sessão logada.
// A lista é o contrato entre quem lê o perfil (o serviço de navegador) e quem
// consome a sessão (o downloader): os dois precisam concordar sobre o que
// significa "esta conta está autenticada".
var sessionCookieNames = map[string]bool{
	"SID":               true,
	"__Secure-1PSID":    true,
	"__Secure-3PSID":    true,
	"LOGIN_INFO":        true,
	"SAPISID":           true,
	"__Secure-1PAPISID": true,
}

// IsSessionCookie informa se o cookie faz parte da sessão autenticada.
func IsSessionCookie(name string) bool {
	return sessionCookieNames[name]
}

// CountSessionCookies conta os cookies de sessão em um jar no formato Netscape.
// Serve para descartar uma conta obviamente deslogada sem gastar uma requisição
// ao YouTube. Nenhum valor de cookie é lido, apenas os nomes.
func CountSessionCookies(jar []byte) int {
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
		if IsSessionCookie(fields[5]) {
			found++
		}
	}

	return found
}
