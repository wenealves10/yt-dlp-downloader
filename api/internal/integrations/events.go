package integrations

import (
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// Os eventos que um sistema integrado pode receber. São strings estáveis e
// fazem parte do contrato público: mudar um valor daqui quebra o `switch` de
// todo cliente já integrado, então um evento novo se ACRESCENTA, nunca se
// renomeia.
const (
	EventDownloadQueued    = "download.queued"
	EventDownloadStarted   = "download.started"
	EventDownloadProgress  = "download.progress"
	EventDownloadCompleted = "download.completed"
	EventDownloadFailed    = "download.failed"
	EventDownloadCanceled  = "download.canceled"
	EventDownloadExpired   = "download.expired"
	EventDownloadRetrying  = "download.retrying"

	// EventPing só existe para o botão "testar" do painel: confirma que a URL
	// responde e que a assinatura está sendo verificada do outro lado, sem
	// depender de um download real acontecer.
	EventPing = "ping"
)

// EventosDeCicloDeVida é o que um webhook sem lista explícita recebe.
//
// O progresso fica de fora de propósito: são dezenas de atualizações por
// download, e um cliente que só quer saber quando o arquivo está pronto não
// deve ter de descartar noventa POSTs para achar o que importa. Quem quer
// acompanhar a barra pede explicitamente.
var EventosDeCicloDeVida = []string{
	EventDownloadQueued,
	EventDownloadStarted,
	EventDownloadCompleted,
	EventDownloadFailed,
	EventDownloadCanceled,
	EventDownloadExpired,
	EventDownloadRetrying,
}

// EventosValidos é o vocabulário aceito no cadastro do webhook. Sem esta
// barreira, um erro de digitação ("download.complete") viraria um webhook que
// nunca dispara e ninguém descobre por quê.
var EventosValidos = append([]string{EventDownloadProgress, EventPing}, EventosDeCicloDeVida...)

// EventoValido diz se o tipo pode ser assinado.
func EventoValido(tipo string) bool {
	for _, valido := range EventosValidos {
		if valido == tipo {
			return true
		}
	}
	return false
}

// TipoDoStatus traduz o status do download no evento correspondente.
//
// `primeiroProcessing` separa "o download começou" de "o download avançou".
// Os dois chegam como PROCESSING no mesmo stream, e sem essa distinção o
// cliente receberia `download.started` a cada atualização de barra — ou nunca
// receberia o início, se tratássemos todo PROCESSING como progresso.
func TipoDoStatus(status db.CoreDownloadStatus, primeiroProcessing bool) string {
	switch status {
	case db.CoreDownloadStatusPENDING:
		return EventDownloadQueued
	case db.CoreDownloadStatusPROCESSING:
		if primeiroProcessing {
			return EventDownloadStarted
		}
		return EventDownloadProgress
	case db.CoreDownloadStatusCOMPLETED:
		return EventDownloadCompleted
	case db.CoreDownloadStatusFAILED:
		return EventDownloadFailed
	case db.CoreDownloadStatusCANCELED:
		return EventDownloadCanceled
	case db.CoreDownloadStatusEXPIRED:
		return EventDownloadExpired
	case db.CoreDownloadStatusRETRYING:
		return EventDownloadRetrying
	}
	return ""
}

// Terminal diz se o evento encerra o ciclo do download. É o que permite ao
// cliente saber quando pode parar de esperar, e a nós saber quando vale a pena
// pagar uma consulta ao banco para montar um payload completo.
func Terminal(tipo string) bool {
	switch tipo {
	case EventDownloadCompleted, EventDownloadFailed,
		EventDownloadCanceled, EventDownloadExpired:
		return true
	}
	return false
}

// Assina responde se este webhook quer este evento.
//
// A lista vazia significa "os de ciclo de vida", e não "nenhum": um webhook
// recém-cadastrado sem escolha explícita precisa funcionar, ou a integração
// nasce muda. Progresso nunca entra por omissão — só com include_progress.
func Assina(eventos []string, incluirProgresso bool, tipo string) bool {
	if tipo == EventDownloadProgress && !incluirProgresso {
		return false
	}

	if len(eventos) == 0 {
		if tipo == EventDownloadProgress {
			return incluirProgresso
		}
		for _, padrao := range EventosDeCicloDeVida {
			if padrao == tipo {
				return true
			}
		}
		return false
	}

	for _, assinado := range eventos {
		if assinado == tipo {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Corpo da notificação
// ---------------------------------------------------------------------------

// Envelope é o corpo JSON que o sistema integrado recebe.
//
// O envelope existe para que o cliente leia SEMPRE os mesmos três campos de
// fora (`id`, `type`, `created_at`) e só então olhe `data`. Enviar o download
// na raiz economizaria um nível e cobraria o preço no dia em que houvesse um
// evento que não é sobre download.
type Envelope struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	CreatedAt     time.Time   `json:"created_at"`
	IntegrationID string      `json:"integration_id"`
	Data          EventoDados `json:"data"`
}

// EventoDados carrega o assunto do evento.
type EventoDados struct {
	Download *DownloadPayload `json:"download,omitempty"`
	// Message só é usado pelo evento de teste.
	Message string `json:"message,omitempty"`
}

// DownloadPayload é a visão PÚBLICA do download, e é a mesma coisa que a API
// devolve em GET /downloads/{id}.
//
// Que seja a mesma não é conveniência: um cliente que recebe o webhook e depois
// consulta a rota tem de ver a mesma forma, ou vai escrever dois parsers para o
// mesmo objeto. E o que está de fora também é deliberado — `provider` e
// `error_detail` descrevem a NOSSA infraestrutura e nunca saem para um sistema
// cliente, exatamente como não saem para um usuário comum.
type DownloadPayload struct {
	ID              string     `json:"id"`
	Status          string     `json:"status"`
	Title           string     `json:"title"`
	OriginalURL     string     `json:"original_url"`
	Platform        string     `json:"platform"`
	PlatformLabel   string     `json:"platform_label,omitempty"`
	Format          string     `json:"format"`
	QualityLabel    string     `json:"quality_label,omitempty"`
	Uploader        string     `json:"uploader,omitempty"`
	ThumbnailURL    string     `json:"thumbnail_url,omitempty"`
	DurationSeconds int32      `json:"duration_seconds,omitempty"`
	FileSizeBytes   int64      `json:"file_size_bytes,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`

	Progress *ProgressPayload `json:"progress,omitempty"`

	// Links diz ao cliente para onde ir buscar o arquivo. É uma rota NOSSA,
	// autenticada pela chave dele — e não a URL assinada do bucket.
	//
	// A URL assinada vive um minuto. Colocá-la aqui entregaria ao cliente um
	// endereço que provavelmente já expirou quando a fila dele processar o
	// evento, e o suporte passaria a explicar isso toda semana.
	Links *DownloadLinks `json:"links,omitempty"`
}

// ProgressPayload é o andamento, presente nos eventos de progresso.
type ProgressPayload struct {
	Percent         float64 `json:"percent"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      int64   `json:"total_bytes"`
	SpeedBPS        int64   `json:"speed_bps"`
	ETASeconds      int     `json:"eta_seconds"`
	Postprocess     bool    `json:"postprocess"`
}

// DownloadLinks reúne as rotas úteis daquele download.
type DownloadLinks struct {
	Self        string `json:"self"`
	DownloadURL string `json:"download_url,omitempty"`
	File        string `json:"file,omitempty"`
}
