package media

import (
	"net"
	"net/url"
	"strings"
)

// maxURLLength corta URLs absurdas antes de qualquer processamento. Nenhum link
// legítimo de vídeo chega perto disso, e o limite evita que uma entrada enorme
// vire argumento de processo.
const maxURLLength = 2048

// NormalizeURL valida e limpa uma URL vinda do usuário. É a fronteira entre
// entrada não confiável e o resto do sistema: tudo que sai daqui já é seguro
// para virar argumento de processo.
//
// Devolve a URL normalizada (para gravar e exibir) e a forma parseada.
func NormalizeURL(bruta string) (string, *url.URL, error) {
	bruta = strings.TrimSpace(bruta)
	if bruta == "" {
		return "", nil, wrap(ErrInvalidURL, "vazia")
	}
	if len(bruta) > maxURLLength {
		return "", nil, wrap(ErrInvalidURL, "excede o tamanho máximo")
	}

	// Caracteres de controle em uma URL não têm uso legítimo e são o material
	// de ataques de injeção em argumento e de quebra de log.
	for _, r := range bruta {
		if r < 0x20 || r == 0x7f {
			return "", nil, wrap(ErrInvalidURL, "contém caractere de controle")
		}
	}

	// Sem esquema, assume https. É o que o usuário faz ao colar "youtu.be/x".
	if !strings.Contains(bruta, "://") {
		bruta = "https://" + bruta
	}

	parsed, err := url.Parse(bruta)
	if err != nil {
		return "", nil, wrap(ErrInvalidURL, err.Error())
	}

	// Só http(s). Sem isto, `file:///etc/passwd` viraria um caminho local
	// entregue ao processo de download.
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", nil, wrap(ErrInvalidURL, "esquema não suportado")
	}

	host := parsed.Hostname()
	if host == "" {
		return "", nil, wrap(ErrInvalidURL, "sem host")
	}

	// Um "-" no começo do host faria a URL inteira parecer uma opção de linha
	// de comando para o processo filho.
	if strings.HasPrefix(parsed.String(), "-") {
		return "", nil, wrap(ErrInvalidURL, "formato inesperado")
	}

	if enderecoInterno(host) {
		return "", nil, wrap(ErrInvalidURL, "endereço de rede interna")
	}

	// Credenciais embutidas (user:senha@host) são descartadas: elas iriam parar
	// no banco, no log e na tela.
	parsed.User = nil
	parsed.Fragment = ""
	parsed.Host = strings.ToLower(parsed.Host)

	return parsed.String(), parsed, nil
}

// enderecoInterno barra o que apontaria para dentro da infraestrutura. Sem
// isto, uma URL como http://169.254.169.254/ ou http://redis:6379/ viraria uma
// requisição feita pelo servidor a partir da rede interna.
//
// Não substitui a política de rede do container: é a primeira barreira, não a
// única.
func enderecoInterno(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))

	switch host {
	case "localhost", "localhost.localdomain", "metadata.google.internal":
		return true
	}
	// Nomes sem ponto só existem dentro da rede do Docker (postgres, redis,
	// browser). Nenhuma plataforma pública tem host assim.
	if !strings.Contains(host, ".") {
		return true
	}
	if strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Nome de domínio comum. A resolução acontece no processo filho, e é a
		// rede do container que precisa impedir a saída para IP privado.
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
