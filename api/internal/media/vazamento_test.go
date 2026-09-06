package media

import (
	"strings"
	"testing"
)

// termosInternos são palavras que denunciam a NOSSA infraestrutura. O cliente
// final não pode saber que existe servidor, provider, sessão ou plataforma
// recusando alguma coisa: para ele a falha é do produto.
var termosInternos = []string{
	"servidor", "provider", "sessão", "sessao", "cookie", "yt-dlp", "ffmpeg",
	"proxy", "container", "worker", "mecanismo", "conta autenticada",
	"stderr", "http error", "403", "429", "região do servidor",
}

// Esta é a regra dura: nenhuma mensagem pública pode citar interno. Um erro
// novo com a frase errada quebra aqui, e não em produção na frente de um
// cliente.
func TestMensagensPublicasNaoCitamInterno(t *testing.T) {
	for erro, mensagem := range mensagensPublicas {
		minuscula := strings.ToLower(mensagem)
		for _, termo := range termosInternos {
			if strings.Contains(minuscula, termo) {
				t.Errorf("a mensagem pública de %v cita %q: %q", erro, termo, mensagem)
			}
		}
		if mensagem == "" {
			t.Errorf("%v ficou sem mensagem pública", erro)
		}
	}
}

// Todo erro de domínio precisa de uma mensagem pública própria. Sem isso ele
// cairia no genérico sem ninguém perceber.
func TestTodoErroDeDominioTemMensagemPublica(t *testing.T) {
	dominio := []error{
		ErrInvalidURL, ErrUnsupportedPlatform, ErrContentUnavailable, ErrContentPrivate,
		ErrAuthRequired, ErrGeoBlocked, ErrLiveContent, ErrFormatUnavailable,
		ErrRateLimited, ErrBlocked, ErrNetwork, ErrTimeout, ErrCanceled,
		ErrProviderUnavailable, ErrDownloadFailed,
	}

	for _, erro := range dominio {
		if _, existe := mensagensPublicas[erro]; !existe {
			t.Errorf("%v não tem mensagem pública", erro)
		}
	}
}

// O caso concreto que motivou a regra: a mensagem de domínio do bloqueio diz
// "a partir deste servidor", e isso não pode chegar ao cliente.
func TestPublicMessageNaoRepeteAMensagemDeDominio(t *testing.T) {
	interno := UserMessage(&Error{Kind: ErrBlocked})
	publico := PublicMessage(&Error{Kind: ErrBlocked})

	if !strings.Contains(interno, "servidor") {
		t.Fatal("a mensagem de domínio deveria ser a precisa, para quem opera")
	}
	if strings.Contains(strings.ToLower(publico), "servidor") {
		t.Errorf("a mensagem pública vazou infraestrutura: %q", publico)
	}
	if publico == interno {
		t.Error("as duas camadas não podem ser a mesma frase")
	}
}

// Um erro desconhecido não pode virar uma mensagem vazia nem repassar o texto
// original, que pode ser qualquer coisa vinda de uma biblioteca.
func TestPublicMessageDegradaParaOGenerico(t *testing.T) {
	publico := PublicMessage(&Error{Kind: ErrDownloadFailed, Detail: "ERROR: HTTP Error 403: Blocked"})

	if publico != mensagensPublicas[ErrDownloadFailed] {
		t.Fatalf("esperado o genérico, obtido %q", publico)
	}
	if strings.Contains(publico, "403") {
		t.Error("o detalhe técnico vazou para a mensagem pública")
	}
	if PublicMessage(nil) != "" {
		t.Error("erro nulo não deveria produzir mensagem")
	}
}

// O detalhe técnico nunca é a mensagem pública, qualquer que seja o erro.
func TestDetalheNuncaViraMensagemPublica(t *testing.T) {
	detalhe := "ERROR: [Reddit] abc: Unable to download webpage: HTTP Error 403: Blocked"

	for _, kind := range []error{ErrBlocked, ErrNetwork, ErrDownloadFailed, ErrProviderUnavailable} {
		publico := PublicMessage(&Error{Kind: kind, Detail: detalhe})
		if strings.Contains(publico, "403") || strings.Contains(publico, "Reddit") {
			t.Errorf("[%v] a mensagem pública vazou o detalhe: %q", kind, publico)
		}
	}
}
