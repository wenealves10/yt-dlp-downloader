package integrations

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Decisao é o resultado de uma consulta ao limitador. Os três números viram os
// headers `X-RateLimit-*` e o `Retry-After` da resposta: sem eles o cliente só
// sabe que levou 429 e a única estratégia que lhe resta é tentar de novo às
// cegas — o que piora exatamente o problema que o limite existe para conter.
type Decisao struct {
	Permitido bool
	Limite    int
	Restante  int
	Espera    time.Duration
}

// Limiter aplica dois tetos por chave de API, e são dois porque contêm abusos
// diferentes:
//
//   - por minuto: o volume contratado. É o número que o cliente vê e planeja.
//   - por segundo: a rajada. Uma janela fixa de um minuto permite gastar a cota
//     inteira nos primeiros milissegundos — e, pior, gastar duas cotas na
//     virada da janela (fim de uma, começo da outra). Um teto por segundo
//     derivado do primeiro fecha essa brecha sem que o cliente precise
//     configurar nada.
type Limiter struct {
	redis *redis.Client
}

func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{redis: client}
}

// rajadaPermitida deriva o teto por segundo do teto por minuto.
//
// Um décimo da cota por segundo deixa o cliente disparar um punhado de
// chamadas em paralelo (o caso real de quem processa uma fila) e ainda impede
// que os 60 pedidos do minuto cheguem todos no mesmo instante. O piso de 5
// evita que um limite baixo vire um limite de 1 por segundo, que travaria até
// o uso normal.
func rajadaPermitida(porMinuto int) int {
	rajada := porMinuto / 10
	if rajada < 5 {
		rajada = 5
	}
	if rajada > porMinuto {
		rajada = porMinuto
	}
	return rajada
}

// Allow conta esta requisição e diz se ela passa.
//
// Falha ABERTA de propósito: se o Redis estiver fora, a alternativa seria
// recusar toda chamada de toda integração por causa de um componente auxiliar,
// o que transformaria uma degradação em indisponibilidade total. O limite de
// downloads simultâneos e a cota diária continuam valendo nesse cenário, porque
// vivem no Postgres — o abuso segue contido no que realmente custa caro.
func (l *Limiter) Allow(ctx context.Context, chave string, porMinuto int) Decisao {
	if porMinuto <= 0 {
		return Decisao{Permitido: true, Limite: porMinuto, Restante: 0}
	}
	if l == nil || l.redis == nil {
		return Decisao{Permitido: true, Limite: porMinuto, Restante: porMinuto}
	}

	agora := time.Now()
	janelaMinuto := fmt.Sprintf("ratelimit:int:%s:m:%d", chave, agora.Unix()/60)
	janelaSegundo := fmt.Sprintf("ratelimit:int:%s:s:%d", chave, agora.Unix())

	pipe := l.redis.Pipeline()
	incrMinuto := pipe.Incr(ctx, janelaMinuto)
	// O TTL é renovado a cada passagem em vez de só na primeira. Um SETNX de
	// expiração que falhasse deixaria a chave eterna, e a partir daí o cliente
	// ficaria permanentemente no teto.
	pipe.Expire(ctx, janelaMinuto, 2*time.Minute)
	incrSegundo := pipe.Incr(ctx, janelaSegundo)
	pipe.Expire(ctx, janelaSegundo, 2*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("integracoes: limitador indisponível para %s: %v", chave, err)
		return Decisao{Permitido: true, Limite: porMinuto, Restante: porMinuto}
	}

	usadoMinuto := int(incrMinuto.Val())
	usadoSegundo := int(incrSegundo.Val())

	restante := porMinuto - usadoMinuto
	if restante < 0 {
		restante = 0
	}

	if usadoSegundo > rajadaPermitida(porMinuto) {
		// Um segundo é o tempo máximo de espera útil aqui: dizer mais faria o
		// cliente dormir sem necessidade.
		return Decisao{Limite: porMinuto, Restante: restante, Espera: time.Second}
	}

	if usadoMinuto > porMinuto {
		// Espera até o fim da janela atual, arredondada para cima: devolver
		// zero convidaria a uma nova tentativa imediata, que também falharia.
		espera := time.Duration(60-agora.Unix()%60) * time.Second
		return Decisao{Limite: porMinuto, Restante: 0, Espera: espera}
	}

	return Decisao{Permitido: true, Limite: porMinuto, Restante: restante}
}

// DeveRegistrarUso limita a frequência da escrita de `last_used_at` da chave.
//
// Sem isto, cada requisição da API viraria também um UPDATE no Postgres só para
// mover um carimbo alguns milissegundos — dobrando a escrita do banco para uma
// informação cuja utilidade é "esta chave andou hoje?". Uma vez por minuto por
// chave responde a mesma pergunta.
func (l *Limiter) DeveRegistrarUso(ctx context.Context, chaveID string) bool {
	if l == nil || l.redis == nil {
		// Sem Redis não há como coordenar: registrar sempre é preferível a
		// perder o rastro de uso por completo.
		return true
	}
	ok, err := l.redis.SetNX(ctx, "intkey:touch:"+chaveID, "1", time.Minute).Result()
	if err != nil {
		return false
	}
	return ok
}

// PrimeiroProcessing diz se este é o primeiro evento PROCESSING do download.
//
// É o que separa `download.started` de `download.progress`, e precisa ser
// coordenado fora do processo: o stream de eventos pode ser consumido por
// qualquer réplica da API, e uma flag em memória faria cada réplica anunciar
// um "começou" próprio para o mesmo download.
//
// O TTL é longo o bastante para cobrir o download mais demorado e curto o
// bastante para não guardar lixo para sempre.
func (l *Limiter) PrimeiroProcessing(ctx context.Context, downloadID string) bool {
	if l == nil || l.redis == nil {
		return false
	}
	ok, err := l.redis.SetNX(ctx, "intwh:started:"+downloadID, "1", 6*time.Hour).Result()
	if err != nil {
		log.Printf("integracoes: falha ao marcar início de %s: %v", downloadID, err)
		return false
	}
	return ok
}

// ProgressoLiberado espaça os eventos de progresso de um mesmo download.
//
// O worker publica progresso várias vezes por segundo. Repassar isso como
// webhook significaria centenas de POSTs por download — uma inundação no
// servidor do cliente, gerada por nós, e uma linha de entrega no banco para
// cada uma. Um evento a cada `intervalo` preserva a utilidade (a barra andando)
// sem o custo.
func (l *Limiter) ProgressoLiberado(ctx context.Context, downloadID string, intervalo time.Duration) bool {
	if l == nil || l.redis == nil {
		return false
	}
	if intervalo <= 0 {
		return true
	}
	ok, err := l.redis.SetNX(ctx, "intwh:prog:"+downloadID, "1", intervalo).Result()
	if err != nil {
		return false
	}
	return ok
}
