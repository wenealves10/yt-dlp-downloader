package integrations

import (
	"net/http"
	"testing"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// Um destino público legítimo tem de passar. Sem este caso, uma checagem
// exagerada demais recusaria toda URL e os testes de recusa acima continuariam
// passando — o webhook simplesmente nunca funcionaria.
func TestURLDeWebhookAceitaDestinoPublico(t *testing.T) {
	// IP literal para não depender de DNS no teste.
	if err := ValidateWebhookURL("https://203.0.113.10/webhooks/advideo", false); err != nil {
		t.Errorf("um endereço público com https deveria ser aceito: %v", err)
	}
	if err := ValidateWebhookURL("https://203.0.113.10:8443/hooks?origem=advideo", false); err != nil {
		t.Errorf("porta e query não deveriam impedir o cadastro: %v", err)
	}
}

// O primeiro PROCESSING é "começou"; os seguintes são "andou". Sem essa
// distinção, o cliente receberia `download.started` a cada atualização de
// barra — ou nunca receberia o início.
func TestTipoDoEventoPorStatus(t *testing.T) {
	casos := []struct {
		status   db.CoreDownloadStatus
		primeiro bool
		esperado string
	}{
		{db.CoreDownloadStatusPENDING, false, EventDownloadQueued},
		{db.CoreDownloadStatusPROCESSING, true, EventDownloadStarted},
		{db.CoreDownloadStatusPROCESSING, false, EventDownloadProgress},
		{db.CoreDownloadStatusCOMPLETED, false, EventDownloadCompleted},
		{db.CoreDownloadStatusFAILED, false, EventDownloadFailed},
		{db.CoreDownloadStatusCANCELED, false, EventDownloadCanceled},
		{db.CoreDownloadStatusEXPIRED, false, EventDownloadExpired},
		{db.CoreDownloadStatusRETRYING, false, EventDownloadRetrying},
	}

	for _, caso := range casos {
		if obtido := TipoDoStatus(caso.status, caso.primeiro); obtido != caso.esperado {
			t.Errorf("TipoDoStatus(%s, primeiro=%v) = %q, esperava %q",
				caso.status, caso.primeiro, obtido, caso.esperado)
		}
	}

	// Um status novo no banco sem tradução aqui não pode virar um evento vazio
	// enviado ao cliente.
	if TipoDoStatus(db.CoreDownloadStatus("ESTADO_NOVO"), false) != "" {
		t.Error("status desconhecido deveria resultar em nenhum evento")
	}
}

func TestEventosTerminais(t *testing.T) {
	terminais := []string{
		EventDownloadCompleted, EventDownloadFailed,
		EventDownloadCanceled, EventDownloadExpired,
	}
	for _, evento := range terminais {
		if !Terminal(evento) {
			t.Errorf("%s encerra o ciclo e deveria ser terminal", evento)
		}
	}

	emCurso := []string{EventDownloadQueued, EventDownloadStarted, EventDownloadProgress, EventPing}
	for _, evento := range emCurso {
		if Terminal(evento) {
			t.Errorf("%s não encerra o ciclo", evento)
		}
	}
}

// A regra da lista vazia: "todos os de ciclo de vida", nunca "nenhum" e nunca
// "todos, inclusive progresso". Um webhook cadastrado sem escolher eventos tem
// de funcionar, e não pode receber dezenas de POSTs por download.
func TestAssinaturaDeEventos(t *testing.T) {
	if !Assina(nil, false, EventDownloadCompleted) {
		t.Error("sem lista explícita, os eventos de ciclo de vida devem ser entregues")
	}
	if Assina(nil, false, EventDownloadProgress) {
		t.Error("progresso nunca deve ser entregue sem include_progress")
	}
	if !Assina(nil, true, EventDownloadProgress) {
		t.Error("com include_progress, o progresso deve ser entregue")
	}
	if Assina(nil, false, EventPing) {
		t.Error("ping é evento de teste e não entra na lista padrão")
	}

	// Com lista explícita, só o que está nela.
	apenasConclusao := []string{EventDownloadCompleted}
	if !Assina(apenasConclusao, false, EventDownloadCompleted) {
		t.Error("o evento assinado explicitamente deveria ser entregue")
	}
	if Assina(apenasConclusao, false, EventDownloadFailed) {
		t.Error("um evento fora da lista explícita não deveria ser entregue")
	}

	// include_progress desligado vence a lista explícita: é a trava que impede
	// inundar o cliente por um cadastro descuidado.
	if Assina([]string{EventDownloadProgress}, false, EventDownloadProgress) {
		t.Error("progresso na lista, mas com include_progress desligado, não deve ser entregue")
	}
	if !Assina([]string{EventDownloadProgress}, true, EventDownloadProgress) {
		t.Error("progresso na lista e include_progress ligado deve ser entregue")
	}
}

func TestVocabularioDeEventos(t *testing.T) {
	for _, evento := range EventosValidos {
		if !EventoValido(evento) {
			t.Errorf("%s está na lista de válidos e deveria ser aceito", evento)
		}
	}
	// O erro de digitação que motivou a validação: um webhook que nunca dispara
	// e uma investigação que começa no lugar errado.
	if EventoValido("download.complete") {
		t.Error("um evento com o nome errado deveria ser recusado no cadastro")
	}
	if EventoValido("") {
		t.Error("evento vazio não é válido")
	}
}

// A classificação da resposta do endpoint decide entre insistir e desistir.
// Errar para o lado de insistir manda o mesmo POST oito vezes a quem já
// recusou; errar para o outro abandona o evento na primeira oscilação de rede.
func TestClassificacaoDaRespostaDoEndpoint(t *testing.T) {
	sucessos := []int{200, 201, 202, 204, 299}
	for _, status := range sucessos {
		if !(Resultado{StatusCode: status}).Sucesso() {
			t.Errorf("HTTP %d deveria contar como entregue", status)
		}
	}

	repetiveis := []int{408, 429, 500, 502, 503, 504}
	for _, status := range repetiveis {
		resultado := Resultado{StatusCode: status}
		if resultado.Sucesso() {
			t.Errorf("HTTP %d não é sucesso", status)
		}
		if !resultado.DeveRepetir() {
			t.Errorf("HTTP %d é indisponibilidade e deveria ser repetido", status)
		}
	}

	recusas := []int{400, 401, 403, 404, 422}
	for _, status := range recusas {
		if (Resultado{StatusCode: status}).DeveRepetir() {
			t.Errorf("HTTP %d é recusa do endpoint; repetir produziria a mesma recusa", status)
		}
	}

	// 410 é a única forma de o cliente desligar a notificação pelo próprio
	// endpoint.
	if !(Resultado{StatusCode: http.StatusGone}).Definitivo() {
		t.Error("410 Gone deveria encerrar as tentativas de vez")
	}
	if (Resultado{StatusCode: 500}).Definitivo() {
		t.Error("500 não é definitivo")
	}

	// Erro de rede é indisponibilidade; destino interno é configuração errada e
	// nunca vai melhorar sozinho.
	if !(Resultado{Erro: http.ErrHandlerTimeout}).DeveRepetir() {
		t.Error("erro de rede/timeout deveria ser repetido")
	}
	if (Resultado{Erro: ErrURLPrivada}).DeveRepetir() {
		t.Error("destino interno não melhora com nova tentativa")
	}
}

// A rajada por segundo é o que impede gastar a cota do minuto inteira em um
// instante — e, na virada da janela, duas cotas seguidas.
func TestRajadaPermitida(t *testing.T) {
	casos := map[int]int{
		60:   6,
		600:  60,
		10:   5, // piso: um décimo daria 1/s e travaria o uso normal
		1:    1, // nunca maior que o próprio limite
		3000: 300,
	}
	for porMinuto, esperado := range casos {
		if obtido := rajadaPermitida(porMinuto); obtido != esperado {
			t.Errorf("rajadaPermitida(%d) = %d, esperava %d", porMinuto, obtido, esperado)
		}
	}
}
