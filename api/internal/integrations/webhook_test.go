package integrations

import (
	"net"
	"strings"
	"testing"
)

// A assinatura é a única prova que o cliente tem de que o POST veio de nós. Os
// quatro casos abaixo são as quatro formas de essa prova falhar.
func TestAssinaturaMudaComTudoQueImporta(t *testing.T) {
	const segredo = "whsec_0123456789abcdef"
	corpo := []byte(`{"type":"download.completed"}`)
	const timestamp = int64(1757700000)

	base := Sign(segredo, timestamp, corpo)

	if !strings.HasPrefix(base, "v1=") {
		t.Errorf("a assinatura precisa declarar a versão do algoritmo: %q", base)
	}

	if Sign(segredo, timestamp+1, corpo) == base {
		t.Error("trocar o timestamp tem de mudar a assinatura; sem isso uma entrega capturada " +
			"poderia ser reenviada amanhã com a assinatura ainda válida")
	}
	if Sign(segredo, timestamp, []byte(`{"type":"download.failed"}`)) == base {
		t.Error("trocar o corpo tem de mudar a assinatura; sem isso o payload poderia ser " +
			"alterado no caminho")
	}
	if Sign("whsec_outro", timestamp, corpo) == base {
		t.Error("trocar o segredo tem de mudar a assinatura")
	}
}

func TestVerificacaoDeAssinatura(t *testing.T) {
	const segredo = "whsec_0123456789abcdef"
	corpo := []byte(`{"id":"abc"}`)
	const timestamp = int64(1757700000)

	assinatura := Sign(segredo, timestamp, corpo)

	if !VerifySignature(segredo, timestamp, corpo, assinatura) {
		t.Fatal("a assinatura que geramos deveria ser verificada como válida")
	}

	// Cada um destes é uma tentativa real de falsificação.
	if VerifySignature(segredo, timestamp, []byte(`{"id":"outro"}`), assinatura) {
		t.Error("corpo trocado foi aceito")
	}
	if VerifySignature(segredo, timestamp+300, corpo, assinatura) {
		t.Error("timestamp trocado foi aceito")
	}
	if VerifySignature("whsec_errado", timestamp, corpo, assinatura) {
		t.Error("segredo errado foi aceito")
	}
	if VerifySignature(segredo, timestamp, corpo, "v1=00") {
		t.Error("assinatura truncada foi aceita")
	}
}

// O teste que importa mais neste arquivo: a URL do webhook é escolhida por quem
// cadastra, e é o SERVIDOR que faz a requisição. Cada caso recusado abaixo é um
// destino que transformaria o cadastro em uma sonda da rede interna (SSRF).
func TestURLDeWebhookRecusaDestinoInterno(t *testing.T) {
	recusadas := map[string]string{
		"vazia":                  "",
		"sem esquema":            "meusistema.com/webhook",
		"http em produção":       "http://meusistema.com/webhook",
		"outro esquema":          "ftp://meusistema.com/webhook",
		"arquivo local":          "file:///etc/passwd",
		"loopback":               "https://127.0.0.1/webhook",
		"localhost por ip v6":    "https://[::1]/webhook",
		"rede privada 10":        "https://10.0.0.5/webhook",
		"rede privada 192.168":   "https://192.168.1.10/webhook",
		"rede privada 172.16":    "https://172.16.0.9/webhook",
		"metadados da instância": "https://169.254.169.254/latest/meta-data/",
		"cgnat":                  "https://100.64.1.1/webhook",
		"credencial na url":      "https://usuario:senha@meusistema.com/webhook",
		"com fragmento":          "https://meusistema.com/webhook#parte",
	}

	for nome, url := range recusadas {
		if err := ValidateWebhookURL(url, false); err == nil {
			t.Errorf("%s deveria ser recusada: %q", nome, url)
		}
	}
}

// Em desenvolvimento o sistema integrado roda na própria máquina. O escape
// hatch tem de funcionar — e tem de ser o ÚNICO caminho para isso.
func TestURLDeWebhookAceitaLocalSomenteComPermissao(t *testing.T) {
	const local = "http://localhost:3000/webhooks/advideo"

	if err := ValidateWebhookURL(local, false); err == nil {
		t.Error("localhost não pode ser aceito sem a permissão explícita")
	}
	if err := ValidateWebhookURL(local, true); err != nil {
		t.Errorf("com a permissão de desenvolvimento, localhost deveria passar: %v", err)
	}
}

func TestClassificacaoDeIP(t *testing.T) {
	casos := map[string]bool{
		"8.8.8.8":              true,
		"203.0.113.10":         true,
		"2001:4860:4860::8888": true,
		"127.0.0.1":            false,
		"::1":                  false,
		"10.1.2.3":             false,
		"172.20.0.1":           false,
		"192.168.0.1":          false,
		"169.254.169.254":      false,
		"100.64.0.1":           false,
		"0.0.0.0":              false,
		"224.0.0.1":            false,
		"240.0.0.1":            false,
	}

	for texto, esperado := range casos {
		if obtido := IPPublico(net.ParseIP(texto)); obtido != esperado {
			t.Errorf("IPPublico(%s) = %v, esperava %v", texto, obtido, esperado)
		}
	}

	if IPPublico(nil) {
		t.Error("um IP que não pôde ser interpretado não pode ser considerado público")
	}
}

// O controle de discagem é o que fecha a janela do DNS rebinding: entre o
// cadastro e a entrega, o nome pode passar a resolver para um endereço interno.
func TestControleDeDiscagemRecusaEnderecoInterno(t *testing.T) {
	controle := ControleDeDiscagem(false)

	if err := controle("tcp", "127.0.0.1:443", nil); err == nil {
		t.Error("discar para loopback deveria ser recusado")
	}
	if err := controle("tcp", "169.254.169.254:80", nil); err == nil {
		t.Error("discar para o endpoint de metadados deveria ser recusado")
	}
	if err := controle("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("discar para endereço público deveria passar: %v", err)
	}

	// Com a permissão de desenvolvimento, nada é barrado.
	if err := ControleDeDiscagem(true)("tcp", "127.0.0.1:3000", nil); err != nil {
		t.Errorf("em desenvolvimento, loopback deveria passar: %v", err)
	}
}

func TestSegredoDeWebhookTemPrefixoEEhUnico(t *testing.T) {
	primeiro, err := GenerateWebhookSecret()
	if err != nil {
		t.Fatalf("geração falhou: %v", err)
	}
	segundo, _ := GenerateWebhookSecret()

	if !strings.HasPrefix(primeiro, WebhookSecretPrefix) {
		t.Errorf("o segredo precisa ser reconhecível pelo prefixo: %q", primeiro)
	}
	if primeiro == segundo {
		t.Error("dois segredos gerados em sequência não podem ser iguais")
	}
}
