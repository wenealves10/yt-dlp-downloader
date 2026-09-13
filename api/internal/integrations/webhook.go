package integrations

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"syscall"
)

const (
	// Nomes dos headers da notificação. Fazem parte do contrato: o cliente lê
	// a assinatura por este nome, e renomear um deles quebra quem já integrou.
	HeaderEvent     = "X-Advideo-Event"
	HeaderDelivery  = "X-Advideo-Delivery"
	HeaderTimestamp = "X-Advideo-Timestamp"
	HeaderSignature = "X-Advideo-Signature"
	HeaderAttempt   = "X-Advideo-Attempt"

	// Prefixo da versão da assinatura. Existe para que um dia seja possível
	// trocar o algoritmo enviando as duas versões no mesmo header, sem quebrar
	// quem só entende a antiga.
	assinaturaVersao = "v1"

	// WebhookSecretPrefix marca o segredo de assinatura, pelo mesmo motivo do
	// prefixo da chave de API: um segredo reconhecível é um segredo que se
	// consegue procurar onde ele não devia estar.
	WebhookSecretPrefix = "whsec_"
)

// GenerateWebhookSecret cria o segredo de assinatura de um webhook.
//
// Diferente da chave de API, este valor é guardado LEGÍVEL no banco — e tem de
// ser: quem verifica a assinatura é o sistema do cliente, com o mesmo segredo
// que usamos para assinar. Um hash aqui tornaria a verificação impossível.
func GenerateWebhookSecret() (string, error) {
	bruto := make([]byte, 32)
	if _, err := rand.Read(bruto); err != nil {
		return "", fmt.Errorf("não foi possível gerar o segredo do webhook: %w", err)
	}
	return WebhookSecretPrefix + hex.EncodeToString(bruto), nil
}

// Sign devolve o valor do header de assinatura.
//
// O timestamp entra DENTRO do que é assinado, e não só num header ao lado. Se
// ficasse fora, um atacante que capturou uma entrega poderia reenviá-la com um
// timestamp novo e a assinatura continuaria válida — o cliente perderia a
// única forma de recusar uma repetição antiga.
func Sign(segredo string, timestamp int64, corpo []byte) string {
	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(corpo)
	return assinaturaVersao + "=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature confere uma assinatura recebida. Não é usada pela API — quem
// verifica é o cliente — mas é a referência executável do algoritmo descrito na
// documentação, e o que garante que o exemplo publicado continue correto.
func VerifySignature(segredo string, timestamp int64, corpo []byte, assinatura string) bool {
	esperada := Sign(segredo, timestamp, corpo)
	// Comparação em tempo constante: um `==` vazaria, pelo tempo de resposta,
	// quantos bytes iniciais o atacante já acertou.
	return hmac.Equal([]byte(esperada), []byte(assinatura))
}

// ---------------------------------------------------------------------------
// Validação da URL de destino
// ---------------------------------------------------------------------------

// ErrURLPrivada é devolvido quando o destino aponta para dentro da nossa rede.
var ErrURLPrivada = errors.New("a URL do webhook aponta para um endereço interno")

// ValidateWebhookURL recusa destinos que transformariam o webhook em uma arma
// contra a nossa própria infraestrutura.
//
// O risco tem nome: SSRF. Quem cadastra a URL escolhe para onde o SERVIDOR vai
// fazer uma requisição autenticada de dentro da rede. Sem esta checagem,
// `http://169.254.169.254/…` (metadados da instância) ou
// `http://postgres:5432` seriam destinos perfeitamente aceitáveis, e o webhook
// serviria para varrer a rede interna a partir de um campo de formulário.
//
// `permitirPrivado` existe só para o ambiente de desenvolvimento, onde o
// sistema integrado roda em localhost.
func ValidateWebhookURL(bruta string, permitirPrivado bool) error {
	bruta = strings.TrimSpace(bruta)
	if bruta == "" {
		return errors.New("informe a URL do webhook")
	}
	if len(bruta) > 2000 {
		return errors.New("a URL do webhook é longa demais")
	}

	parsed, err := url.Parse(bruta)
	if err != nil {
		return errors.New("a URL do webhook é inválida")
	}

	switch parsed.Scheme {
	case "https":
	case "http":
		// HTTP em produção mandaria o payload — e a assinatura — em claro pela
		// internet. Só é tolerado onde a rede é a própria máquina.
		if !permitirPrivado {
			return errors.New("a URL do webhook precisa usar https")
		}
	default:
		return errors.New("a URL do webhook precisa usar https")
	}

	if parsed.User != nil {
		// Credencial na URL vaza no log de toda tentativa de entrega.
		return errors.New("a URL do webhook não pode conter usuário e senha")
	}
	if parsed.Hostname() == "" {
		return errors.New("a URL do webhook precisa ter um host")
	}
	if parsed.Fragment != "" {
		return errors.New("a URL do webhook não pode ter fragmento (#)")
	}

	if permitirPrivado {
		return nil
	}
	return hostPublico(parsed.Hostname())
}

// hostPublico resolve o nome e recusa se QUALQUER endereço retornado for
// interno.
//
// Recusar por "qualquer" e não por "todos" é intencional: um nome que resolve
// para um IP público e outro privado seria aceito na checagem e usado no
// endereço privado na hora de conectar.
func hostPublico(host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if !IPPublico(ip) {
			return ErrURLPrivada
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("não foi possível resolver o host do webhook: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("o host do webhook não resolve para nenhum endereço")
	}
	for _, ip := range ips {
		if !IPPublico(ip) {
			return ErrURLPrivada
		}
	}
	return nil
}

// IPPublico diz se o endereço é roteável na internet.
func IPPublico(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() ||
		ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}

	// 100.64.0.0/10 (CGNAT) não é coberto por IsPrivate e é exatamente a faixa
	// usada por rede de provedor e por algumas redes de container.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return false
		}
		// 0.0.0.0/8 e 240.0.0.0/4 (reservado).
		if v4[0] == 0 || v4[0] >= 240 {
			return false
		}
	}
	return true
}

// ControleDeDiscagem é o `Control` do dialer usado na entrega.
//
// A validação no cadastro confere o nome UMA vez; entre ela e a entrega o DNS
// pode passar a responder outra coisa — é o ataque de rebinding, e é barato de
// executar contra quem só valida na entrada. Esta função roda com o IP que
// está sendo discado de fato, que é o único momento em que a checagem não pode
// ser contornada.
func ControleDeDiscagem(permitirPrivado bool) func(rede, endereco string, c syscall.RawConn) error {
	return func(rede, endereco string, _ syscall.RawConn) error {
		if permitirPrivado {
			return nil
		}
		host, _, err := net.SplitHostPort(endereco)
		if err != nil {
			return fmt.Errorf("endereço de destino inválido: %w", err)
		}
		if !IPPublico(net.ParseIP(host)) {
			return ErrURLPrivada
		}
		return nil
	}
}
