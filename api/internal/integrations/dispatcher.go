package integrations

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
)

const (
	// Tamanho da fila interna do despachante. Generoso de propósito: quem
	// alimenta esta fila é a mesma goroutine que serve o SSE da tela, e o custo
	// de um buffer grande em memória é irrelevante comparado ao de atrasar o
	// tempo real do painel.
	filaDespacho = 2048

	// Quantas entregas são preparadas em paralelo. Cada uma faz um INSERT e um
	// enfileiramento; quatro dão folga sem disputar o pool do Postgres com as
	// requisições HTTP, que têm prioridade.
	trabalhadoresDespacho = 4

	// Por quanto tempo a integração de um usuário fica em memória. O preço de
	// um valor velho aqui é pequeno (um webhook recém-criado pode demorar até
	// este tempo para começar a receber), e o ganho é não consultar o Postgres
	// a cada evento de progresso de cada download da plataforma.
	ttlCacheIntegracao = 60 * time.Second
	ttlCacheWebhooks   = 30 * time.Second

	// Espera máxima ao enfileirar um evento TERMINAL quando a fila interna está
	// cheia. Um "concluído" perdido é o pior desfecho possível para o cliente —
	// ele fica esperando para sempre —, então vale bloquear um instante. Os
	// eventos de progresso, ao contrário, são descartados na hora: o próximo
	// vem em segundos.
	esperaEventoTerminal = 250 * time.Millisecond
)

// TaskWebhookDeliver é o tipo da tarefa que entrega a notificação. O valor
// canônico vive em internal/tasks, junto com todos os outros tipos do projeto;
// o alias aqui evita que quem lê este arquivo precise abrir outro para saber
// qual tarefa é enfileirada.
const TaskWebhookDeliver = tasks.TypeIntegrationWebhook

// PayloadEntrega é o corpo da tarefa. Carrega só o identificador: o conteúdo da
// notificação já está gravado, e repeti-lo na fila abriria a chance de os dois
// divergirem numa reentrega.
type PayloadEntrega struct {
	DeliveryID string `json:"delivery_id"`
}

// DispatcherConfig separa o que é ajustável do que é regra.
type DispatcherConfig struct {
	// BaseURL é a URL pública da API, usada para montar os links do payload.
	BaseURL string
	// IntervaloProgresso espaça os eventos de progresso de um mesmo download.
	IntervaloProgresso time.Duration
}

// Dispatcher transforma evento de download em entrega de webhook.
//
// Ele se pendura no stream que JÁ existe — o mesmo que alimenta o SSE da tela —
// em vez de pedir que cada job avise os webhooks. Essa escolha é o que mantém
// jobs.* sem nenhuma noção de integração: qualquer transição de status que
// apareça na tela do usuário chega ao sistema integrado pelo mesmo caminho, e
// um estado novo no futuro não precisa ser publicado em dois lugares.
type Dispatcher struct {
	store   db.Store
	queue   *asynq.Client
	limiter *Limiter
	config  DispatcherConfig

	fila chan stream.DownloadEvent

	mu             sync.RWMutex
	cacheUsuario   map[uuid.UUID]entradaIntegracao
	cacheWebhooks  map[uuid.UUID]entradaWebhooks
	iniciado       bool
	descartadosLog time.Time
}

type entradaIntegracao struct {
	integracao db.Integration
	existe     bool
	expira     time.Time
}

type entradaWebhooks struct {
	webhooks []db.IntegrationWebhook
	expira   time.Time
}

func NewDispatcher(store db.Store, queue *asynq.Client, limiter *Limiter, config DispatcherConfig) *Dispatcher {
	if config.IntervaloProgresso <= 0 {
		config.IntervaloProgresso = 5 * time.Second
	}
	return &Dispatcher{
		store:         store,
		queue:         queue,
		limiter:       limiter,
		config:        config,
		fila:          make(chan stream.DownloadEvent, filaDespacho),
		cacheUsuario:  make(map[uuid.UUID]entradaIntegracao),
		cacheWebhooks: make(map[uuid.UUID]entradaWebhooks),
	}
}

// Start sobe os trabalhadores. Chamar mais de uma vez é inofensivo.
func (d *Dispatcher) Start(ctx context.Context) {
	if d == nil || d.queue == nil {
		return
	}
	d.mu.Lock()
	if d.iniciado {
		d.mu.Unlock()
		return
	}
	d.iniciado = true
	d.mu.Unlock()

	for i := 0; i < trabalhadoresDespacho; i++ {
		go d.consumir(ctx)
	}
	log.Printf("integracoes: despachante de webhooks iniciado com %d trabalhadores", trabalhadoresDespacho)
}

// Handle recebe o evento do stream. NUNCA bloqueia por muito tempo: quem chama
// é a goroutine que também publica no SSE, e travar aqui congelaria a tela de
// todos os usuários por conta de um webhook.
func (d *Dispatcher) Handle(evento stream.DownloadEvent) {
	if d == nil || d.queue == nil || evento.UserID == "" {
		return
	}

	select {
	case d.fila <- evento:
		return
	default:
	}

	// Fila cheia. Progresso se descarta sem cerimônia; desfecho merece uma
	// espera curta.
	if evento.Progress != nil && !terminalDoStatus(evento.Status) {
		d.registrarDescarte(evento)
		return
	}

	timer := time.NewTimer(esperaEventoTerminal)
	defer timer.Stop()
	select {
	case d.fila <- evento:
	case <-timer.C:
		d.registrarDescarte(evento)
	}
}

// registrarDescarte avisa no log, com no máximo uma linha por segundo: uma fila
// cheia geraria milhares de linhas idênticas e afogaria o que importa.
func (d *Dispatcher) registrarDescarte(evento stream.DownloadEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if time.Since(d.descartadosLog) < time.Second {
		return
	}
	d.descartadosLog = time.Now()
	log.Printf("integracoes: fila de webhooks cheia; evento descartado download=%s status=%s",
		evento.ID, evento.Status)
}

func (d *Dispatcher) consumir(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evento := <-d.fila:
			d.processar(ctx, evento)
		}
	}
}

func (d *Dispatcher) processar(ctx context.Context, evento stream.DownloadEvent) {
	userID, err := uuid.Parse(evento.UserID)
	if err != nil {
		return
	}

	integracao, existe := d.integracaoDe(ctx, userID)
	if !existe || !integracao.Active {
		// O caso esmagadoramente mais comum: o download é de uma pessoa, não de
		// um sistema. Sai antes de qualquer trabalho.
		return
	}

	webhooks := d.webhooksDe(ctx, integracao.ID)
	if len(webhooks) == 0 {
		return
	}

	// A marcação do "primeiro PROCESSING" só é consumida quando há alguém
	// escutando. Fazê-la antes gastaria o marcador em uma integração sem
	// webhook, e o primeiro webhook cadastrado no meio de um download nunca
	// veria o `download.started`.
	primeiro := false
	if evento.Status == db.CoreDownloadStatusPROCESSING {
		primeiro = d.limiter.PrimeiroProcessing(ctx, evento.ID)
	}

	tipo := TipoDoStatus(evento.Status, primeiro)
	if tipo == "" {
		return
	}

	interessados := make([]db.IntegrationWebhook, 0, len(webhooks))
	for _, webhook := range webhooks {
		if Assina(webhook.Events, webhook.IncludeProgress, tipo) {
			interessados = append(interessados, webhook)
		}
	}
	if len(interessados) == 0 {
		return
	}

	if tipo == EventDownloadProgress &&
		!d.limiter.ProgressoLiberado(ctx, evento.ID, d.config.IntervaloProgresso) {
		return
	}

	payload := d.montarPayload(ctx, evento, tipo)

	for _, webhook := range interessados {
		if err := d.Enfileirar(ctx, webhook, integracao.ID, tipo, payload); err != nil {
			log.Printf("integracoes: falha ao preparar entrega webhook=%s evento=%s: %v",
				webhook.ID, tipo, err)
		}
	}
}

// montarPayload monta o corpo da notificação.
//
// Em evento terminal vale uma ida ao banco: o evento do stream é enxuto (basta
// para a tela, que já tem o resto) e o cliente precisa do desfecho completo —
// tamanho do arquivo, validade, duração. Em progresso, o evento basta: são
// muitos, e uma consulta em cada um multiplicaria a carga do Postgres pelo
// número de downloads em andamento.
func (d *Dispatcher) montarPayload(ctx context.Context, evento stream.DownloadEvent, tipo string) DownloadPayload {
	if Terminal(tipo) {
		if id, err := uuid.Parse(evento.ID); err == nil {
			consultaCtx, cancelar := context.WithTimeout(ctx, 5*time.Second)
			defer cancelar()
			if download, err := d.store.GetDownloadByID(consultaCtx, id); err == nil {
				return MontarDownloadPayload(download, d.config.BaseURL)
			}
		}
	}
	return d.payloadDoEvento(evento)
}

// payloadDoEvento monta o corpo com o que o stream trouxe, sem tocar no banco.
func (d *Dispatcher) payloadDoEvento(evento stream.DownloadEvent) DownloadPayload {
	payload := DownloadPayload{
		ID:              evento.ID,
		Status:          string(evento.Status),
		Title:           evento.Title,
		OriginalURL:     evento.OriginalUrl,
		Platform:        evento.Platform,
		PlatformLabel:   evento.PlatformLabel,
		Format:          string(evento.Format),
		ThumbnailURL:    evento.ThumbnailUrl,
		DurationSeconds: evento.DurationSeconds,
		ErrorMessage:    evento.ErrorMessage,
		CreatedAt:       evento.CreatedAt,
		ExpiresAt:       evento.ExpiresAt,
	}

	if evento.Progress != nil {
		payload.Progress = &ProgressPayload{
			Percent:         evento.Progress.Percent,
			DownloadedBytes: evento.Progress.DownloadedBytes,
			TotalBytes:      evento.Progress.TotalBytes,
			SpeedBPS:        evento.Progress.SpeedBPS,
			ETASeconds:      evento.Progress.ETASeconds,
			Postprocess:     evento.Progress.Postprocess,
		}
	}

	if d.config.BaseURL != "" && evento.ID != "" {
		payload.Links = &DownloadLinks{
			Self: d.config.BaseURL + "/v1/integration/downloads/" + evento.ID,
		}
	}
	return payload
}

// Enfileirar grava a entrega e coloca a tarefa na fila.
//
// A ordem importa: a linha é gravada ANTES do enfileiramento. Se o Redis
// recusar, a entrega existe no banco marcada como pendente e aparece no painel
// como "não saiu" — que é a verdade. Enfileirar primeiro e falhar no INSERT
// deixaria o worker buscando uma entrega que não existe.
//
// É público porque o painel usa o mesmo caminho para o webhook de teste e para
// a reentrega manual: um segundo caminho de envio seria um segundo lugar onde a
// assinatura pode sair errada.
func (d *Dispatcher) Enfileirar(
	ctx context.Context,
	webhook db.IntegrationWebhook,
	integrationID uuid.UUID,
	tipo string,
	payload DownloadPayload,
) error {
	entregaID := uuid.New()

	envelope := Envelope{
		ID:            entregaID.String(),
		Type:          tipo,
		CreatedAt:     time.Now().UTC(),
		IntegrationID: integrationID.String(),
		Data:          EventoDados{Download: &payload},
	}
	if tipo == EventPing {
		envelope.Data = EventoDados{
			Message: "Teste de entrega. Se você está lendo isto, a URL e a assinatura estão corretas.",
		}
	}

	corpo, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	var downloadID pgtype.UUID
	if payload.ID != "" {
		if id, err := uuid.Parse(payload.ID); err == nil {
			downloadID = pgtype.UUID{Bytes: id, Valid: true}
		}
	}

	if _, err := d.store.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
		ID:            entregaID,
		WebhookID:     webhook.ID,
		IntegrationID: integrationID,
		EventType:     tipo,
		DownloadID:    downloadID,
		Payload:       corpo,
	}); err != nil {
		return err
	}

	tarefa, err := NovaTarefaEntrega(entregaID.String())
	if err != nil {
		return err
	}

	// MaxRetry aqui é a política de reentrega automática: o asynq já faz
	// backoff exponencial entre as tentativas, e reimplementar isso com uma
	// coluna `next_attempt_at` e um job varredor seria refazer, pior, o que a
	// fila já garante.
	if _, err := d.queue.EnqueueContext(ctx, tarefa,
		asynq.Queue(FilaWebhook),
		asynq.MaxRetry(MaxTentativasEntrega),
		asynq.Timeout(TimeoutEntrega)); err != nil {
		return err
	}
	return nil
}

// NovaTarefaEntrega monta a tarefa do worker.
func NovaTarefaEntrega(deliveryID string) (*asynq.Task, error) {
	corpo, err := json.Marshal(PayloadEntrega{DeliveryID: deliveryID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskWebhookDeliver, corpo), nil
}

// InvalidarCache derruba o que está em memória para uma integração. O painel
// chama depois de mexer em webhook, para que o efeito seja imediato em vez de
// esperar o TTL — quem acabou de salvar uma URL vai testar na hora.
func (d *Dispatcher) InvalidarCache(integrationID, userID uuid.UUID) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.cacheWebhooks, integrationID)
	delete(d.cacheUsuario, userID)
}

// integracaoDe descobre se o download pertence a uma integração.
//
// O cache guarda também a AUSÊNCIA. Sem isso, todo evento de todo usuário comum
// — que é a maioria absoluta do tráfego — viraria uma consulta ao Postgres que
// sempre responde "não tem".
func (d *Dispatcher) integracaoDe(ctx context.Context, userID uuid.UUID) (db.Integration, bool) {
	d.mu.RLock()
	entrada, encontrada := d.cacheUsuario[userID]
	d.mu.RUnlock()
	if encontrada && time.Now().Before(entrada.expira) {
		return entrada.integracao, entrada.existe
	}

	consultaCtx, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()

	integracao, err := d.store.GetIntegrationByUserID(consultaCtx, userID)
	nova := entradaIntegracao{
		integracao: integracao,
		existe:     err == nil,
		expira:     time.Now().Add(ttlCacheIntegracao),
	}
	if err != nil && !ehSemLinha(err) {
		// Erro de banco não é o mesmo que "não é integração": guardar a
		// ausência aqui silenciaria os webhooks por um minuto por causa de uma
		// falha passageira.
		log.Printf("integracoes: falha ao resolver integração do usuário %s: %v", userID, err)
		return db.Integration{}, false
	}

	d.mu.Lock()
	d.cacheUsuario[userID] = nova
	d.mu.Unlock()

	return nova.integracao, nova.existe
}

func (d *Dispatcher) webhooksDe(ctx context.Context, integrationID uuid.UUID) []db.IntegrationWebhook {
	d.mu.RLock()
	entrada, encontrada := d.cacheWebhooks[integrationID]
	d.mu.RUnlock()
	if encontrada && time.Now().Before(entrada.expira) {
		return entrada.webhooks
	}

	consultaCtx, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()

	webhooks, err := d.store.ListActiveIntegrationWebhooks(consultaCtx, integrationID)
	if err != nil {
		log.Printf("integracoes: falha ao listar webhooks de %s: %v", integrationID, err)
		return nil
	}

	d.mu.Lock()
	d.cacheWebhooks[integrationID] = entradaWebhooks{
		webhooks: webhooks,
		expira:   time.Now().Add(ttlCacheWebhooks),
	}
	d.mu.Unlock()

	return webhooks
}

// terminalDoStatus é a versão para o status cru, usada na decisão de descarte,
// antes de o tipo do evento ser resolvido.
func terminalDoStatus(status db.CoreDownloadStatus) bool {
	switch status {
	case db.CoreDownloadStatusCOMPLETED, db.CoreDownloadStatusFAILED,
		db.CoreDownloadStatusCANCELED, db.CoreDownloadStatusEXPIRED:
		return true
	}
	return false
}
