package integrations

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
)

const (
	// FilaWebhook isola a entrega de notificações das filas de download. Sem
	// essa separação, um cliente com endpoint lento consumiria os slots do
	// worker que deveriam estar baixando vídeo.
	FilaWebhook = queues.TypeIntegrationWebhookQueue

	// MaxTentativasEntrega é quantas vezes o asynq reenvia antes de desistir.
	// Com o backoff exponencial padrão, oito tentativas cobrem algumas horas —
	// tempo de sobra para um deploy do lado do cliente terminar, sem insistir
	// por dias em um endereço que mudou.
	MaxTentativasEntrega = 8

	// TimeoutEntrega é o teto por tentativa. Um endpoint que demora mais que
	// isto está com problema, e esperar por ele só ocuparia o worker.
	TimeoutEntrega = 20 * time.Second

	// FalhasParaDesligar é quantas ENTREGAS ABANDONADAS seguidas desligam o
	// webhook. Abandonada, e não tentativa: cada entrega já esgotou as oito
	// tentativas com backoff antes de contar aqui, o que leva horas. Dez
	// seguidas não é oscilação — é endereço morto, e insistir só gera carga e
	// log que ninguém lê. O painel mostra o motivo do desligamento.
	FalhasParaDesligar = 10

	// Quanto do corpo da resposta é guardado para diagnóstico. O suficiente
	// para ver a mensagem de erro do cliente, pouco o bastante para um endpoint
	// que responde HTML de 2 MB não inflar o banco.
	maxCorpoResposta = 2048
)

// Resultado descreve o que aconteceu em uma tentativa de entrega.
type Resultado struct {
	StatusCode int
	Duracao    time.Duration
	Corpo      string
	Erro       error
}

// Sucesso considera entregue qualquer 2xx.
//
// Não exigimos 200 exato: um cliente que responde 202 (aceito, vou processar) ou
// 204 (sem conteúdo) está se comportando corretamente, e recusar isso obrigaria
// metade das integrações a mudar o endpoint por um detalhe sem importância.
func (r Resultado) Sucesso() bool {
	return r.Erro == nil && r.StatusCode >= 200 && r.StatusCode < 300
}

// DeveRepetir decide se vale a pena tentar de novo.
//
// A regra separa "não conseguiu" de "não quis":
//
//   - erro de rede, timeout, 5xx, 408 e 429 são indisponibilidade — o mesmo
//     POST pode dar certo em cinco minutos, então repete.
//   - os outros 4xx são recusa: o endpoint recebeu, entendeu e disse não.
//     Repetir idêntico produziria a mesma recusa mais sete vezes, e a única
//     coisa que resolve é alguém corrigir a configuração — que é justamente o
//     que o contador de falhas do painel vai mostrar.
func (r Resultado) DeveRepetir() bool {
	if r.Erro != nil {
		return !errors.Is(r.Erro, ErrURLPrivada)
	}
	if r.StatusCode == http.StatusRequestTimeout ||
		r.StatusCode == http.StatusTooManyRequests {
		return true
	}
	return r.StatusCode >= 500
}

// Definitivo indica que o destino pediu para não ser mais chamado.
//
// 410 Gone é a única resposta com esse significado inequívoco no HTTP, e
// respeitá-la é o que permite a um cliente desligar a integração pelo próprio
// endpoint, sem depender de nós.
func (r Resultado) Definitivo() bool {
	return r.Erro == nil && r.StatusCode == http.StatusGone
}

// Entregador faz o POST no endpoint do cliente.
//
// O cliente HTTP é montado uma vez e reaproveitado: criar um por entrega
// descartaria o pool de conexões e pagaria handshake TLS completo em cada
// notificação.
type Entregador struct {
	client          *http.Client
	permitirPrivado bool
}

// NewEntregador monta o cliente HTTP das entregas.
//
// Três decisões de segurança estão aqui, e nenhuma é opcional:
//
//  1. Redirecionamento NÃO é seguido. Um endpoint que responde 302 para
//     `http://169.254.169.254/` transformaria a entrega em SSRF, contornando a
//     validação feita no cadastro da URL.
//  2. O dialer confere o IP de destino na hora de conectar, o que fecha a
//     janela do DNS rebinding (validar o nome no cadastro e ele passar a
//     resolver para um endereço interno depois).
//  3. Há timeout em cada etapa, não só no total: um endpoint que aceita a
//     conexão e nunca responde prenderia o worker até o timeout global.
func NewEntregador(timeout time.Duration, permitirPrivado bool) *Entregador {
	if timeout <= 0 {
		timeout = TimeoutEntrega
	}

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   ControleDeDiscagem(permitirPrivado),
	}

	transporte := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConnsPerHost:   4,
		// Proxy do ambiente é ignorado: a entrega vai direto ao cliente, e um
		// proxy configurado por engano no host mandaria o payload assinado para
		// um terceiro.
		Proxy: nil,
	}

	return &Entregador{
		permitirPrivado: permitirPrivado,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transporte,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("o endpoint do webhook respondeu com redirecionamento, que não é seguido")
			},
		},
	}
}

// Entregar envia uma tentativa e descreve o que voltou.
func (e *Entregador) Entregar(
	ctx context.Context,
	destino, segredo, tipo, deliveryID string,
	tentativa int,
	corpo []byte,
) Resultado {
	inicio := time.Now()

	// Revalidar a URL a cada tentativa, e não só no cadastro: entre o cadastro
	// e esta entrega o destino pode ter mudado de DNS, e uma reentrega dias
	// depois é exatamente o cenário em que isso acontece.
	if err := ValidateWebhookURL(destino, e.permitirPrivado); err != nil {
		return Resultado{Erro: err, Duracao: time.Since(inicio)}
	}

	timestamp := time.Now().Unix()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(corpo))
	if err != nil {
		return Resultado{Erro: err, Duracao: time.Since(inicio)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "adVideo-Webhooks/1.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set(HeaderEvent, tipo)
	req.Header.Set(HeaderDelivery, deliveryID)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(HeaderSignature, Sign(segredo, timestamp, corpo))
	req.Header.Set(HeaderAttempt, strconv.Itoa(tentativa))

	resposta, err := e.client.Do(req)
	if err != nil {
		return Resultado{Erro: err, Duracao: time.Since(inicio)}
	}
	defer resposta.Body.Close()

	// O corpo é lido (limitado) mesmo quando não interessa: sem drenar, a
	// conexão não volta para o pool e cada entrega abriria uma nova.
	lido, _ := io.ReadAll(io.LimitReader(resposta.Body, maxCorpoResposta))

	return Resultado{
		StatusCode: resposta.StatusCode,
		Duracao:    time.Since(inicio),
		Corpo:      strings.TrimSpace(string(lido)),
	}
}

// ehSemLinha distingue "não existe" de "o banco falhou". Tratar os dois igual
// faria uma falha de conexão parecer ausência de integração, e os webhooks
// simplesmente pararia de sair sem nada no log.
func ehSemLinha(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
