// Package media define o contrato entre a aplicação e os mecanismos de
// download. Nada aqui sabe o que é yt-dlp: quem conhece o binário é a
// implementação em media/ytdlp, e trocá-la não deve alcançar handlers, jobs,
// banco ou frontend.
package media

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Kind é o tipo de saída pedido pelo usuário. É deliberadamente pequeno: o que
// varia entre plataformas são os formatos concretos, não a intenção.
type Kind string

const (
	KindVideo Kind = "video"
	KindAudio Kind = "audio"
)

// Format é um formato normalizado. Cada provider traduz o vocabulário da sua
// plataforma para cá — o resto da aplicação nunca vê o formato bruto.
type Format struct {
	// ID é o seletor que o provider entende de volta. Opaco para a aplicação:
	// nunca interprete nem construa este valor fora do provider.
	ID string `json:"id"`
	// Label é o que aparece na tela ("1080p", "Áudio 128 kbps").
	Label      string `json:"label"`
	Kind       Kind   `json:"kind"`
	Ext        string `json:"ext"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	FPS        int    `json:"fps,omitempty"`
	VideoCodec string `json:"video_codec,omitempty"`
	AudioCodec string `json:"audio_codec,omitempty"`
	// SizeBytes é estimado quando a plataforma não informa o tamanho exato;
	// SizeApproximate diz qual dos dois casos é.
	SizeBytes       int64 `json:"size_bytes,omitempty"`
	SizeApproximate bool  `json:"size_approximate,omitempty"`
}

// Metadata é o que a aplicação sabe sobre um conteúdo antes de baixá-lo.
type Metadata struct {
	Platform    Platform `json:"platform"`
	Provider    string   `json:"provider"`
	ContentID   string   `json:"content_id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Thumbnail   string   `json:"thumbnail,omitempty"`
	Duration    int      `json:"duration_seconds"`
	IsLive      bool     `json:"is_live"`

	// Autor do conteúdo. UploaderURL vira um link clicável na tela, então
	// passa por SafeExternalURL antes de sair do provider: ela vem da
	// plataforma, e um `javascript:` aqui viraria execução no navegador de
	// quem clica.
	Uploader      string `json:"uploader,omitempty"`
	UploaderID    string `json:"uploader_id,omitempty"`
	UploaderURL   string `json:"uploader_url,omitempty"`
	FollowerCount int64  `json:"follower_count,omitempty"`

	// Números públicos do conteúdo, quando a plataforma informa.
	ViewCount  int64  `json:"view_count,omitempty"`
	LikeCount  int64  `json:"like_count,omitempty"`
	UploadDate string `json:"upload_date,omitempty"` // AAAA-MM-DD
	// WebpageURL é a página canônica; difere da URL colada quando havia
	// parâmetros de rastreamento ou era um encurtador.
	WebpageURL string `json:"webpage_url,omitempty"`

	Formats   []Format  `json:"formats"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Request é o pedido de download já resolvido: URL validada, formato escolhido
// e o diretório onde o arquivo deve nascer.
type Request struct {
	URL string
	// FormatID vazio significa "use o padrão do Kind".
	FormatID string
	Kind     Kind
	// OutputDir é criado e limpo por quem chama; o provider só escreve dentro.
	OutputDir string
	// Filename é o nome base, sem extensão. O provider decide a extensão final.
	Filename string
	// CookieFile é opcional e, quando presente, é um arquivo temporário de
	// sessão. O provider nunca deve copiá-lo nem registrá-lo em log.
	CookieFile string
}

// Result descreve o arquivo produzido.
type Result struct {
	FilePath  string
	SizeBytes int64
	Ext       string
	Duration  time.Duration
}

// Progress é um instantâneo do andamento. Todos os campos são de melhor
// esforço: nem toda plataforma informa tamanho total ou ETA.
type Progress struct {
	Percent         float64 `json:"percent"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      int64   `json:"total_bytes"`
	SpeedBPS        int64   `json:"speed_bps"`
	ETASeconds      int     `json:"eta_seconds"`
	// Postprocess indica a fase depois do download bruto (merge de faixas,
	// conversão de áudio), quando não há mais percentual de bytes.
	Postprocess bool `json:"postprocess"`
}

// ProgressFunc recebe cada atualização. Deve retornar rápido: ela roda na
// leitura do stdout do processo, e bloquear aqui trava o download.
type ProgressFunc func(Progress)

// Role é o papel do processo que hospeda o provider. Existe porque as
// dependências não são as mesmas: quem só resolve metadados nunca executa
// ffmpeg, e cobrá-lo ali reportaria uma falha que não existe.
type Role string

const (
	// RoleResolver só busca metadados: um `--dump-single-json` não junta faixa
	// nem converte áudio.
	RoleResolver Role = "resolver"
	// RoleDownloader baixa e pós-processa. Aqui o ffmpeg é obrigatório.
	RoleDownloader Role = "downloader"
)

func (r Role) Label() string {
	if r == RoleDownloader {
		return "Download"
	}
	return "Resolução de metadados"
}

// Health é o diagnóstico de um provider, usado pelo painel administrativo.
type Health struct {
	Provider  string    `json:"provider"`
	Role      Role      `json:"role,omitempty"`
	Available bool      `json:"available"`
	Version   string    `json:"version,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	Tools     []Tool    `json:"tools,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// HealthReport é o diagnóstico de um processo, publicado para que outro possa
// lê-lo. O painel roda na API, mas o download acontece no worker: sem isso, a
// tela diagnosticaria o container errado.
type HealthReport struct {
	Role       Role      `json:"role"`
	Providers  []Health  `json:"providers"`
	ReportedAt time.Time `json:"reported_at"`
}

// HealthKey é onde o relatório de um papel é publicado. Fica aqui para que
// quem escreve e quem lê não divirjam.
func HealthKey(role Role) string {
	return "media:health:" + string(role)
}

// HealthTTL faz o relatório expirar sozinho: um worker que parou de publicar
// precisa aparecer como silencioso, e não como saudável para sempre.
const HealthTTL = 10 * time.Minute

// Tool é uma dependência externa do provider (o próprio binário, ffmpeg, o
// runtime JavaScript).
type Tool struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	// Required distingue o que impede o provider de funcionar do que apenas
	// reduz a qualidade do resultado.
	Required bool `json:"required"`
}

// Erros de domínio. Os providers traduzem a saída bruta para um destes, e é
// isto que chega ao usuário — a saída crua do processo fica só no log.
var (
	ErrInvalidURL          = errors.New("a URL informada não é válida")
	ErrUnsupportedPlatform = errors.New("esta plataforma ainda não é suportada")
	ErrContentUnavailable  = errors.New("o conteúdo não está disponível")
	ErrContentPrivate      = errors.New("o conteúdo é privado")
	ErrAuthRequired        = errors.New("o conteúdo exige uma conta autenticada")
	ErrGeoBlocked          = errors.New("o conteúdo não está disponível nesta região")
	ErrLiveContent         = errors.New("transmissões ao vivo não podem ser baixadas")
	ErrFormatUnavailable   = errors.New("o formato escolhido não está disponível")
	ErrRateLimited         = errors.New("a plataforma pediu para tentar novamente mais tarde")
	ErrTimeout             = errors.New("o download excedeu o tempo limite")
	ErrCanceled            = errors.New("o download foi cancelado")
	ErrProviderUnavailable = errors.New("o mecanismo de download está indisponível")
	ErrDownloadFailed      = errors.New("não foi possível concluir o download")
)

// Error carrega o erro de domínio junto do detalhe técnico. O detalhe existe
// para o log e para o painel administrativo; a mensagem de `Unwrap` é o que
// pode ser mostrado ao usuário.
type Error struct {
	Kind   error
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Kind.Error()
	}
	return fmt.Sprintf("%s: %s", e.Kind.Error(), e.Detail)
}

func (e *Error) Unwrap() error { return e.Kind }

// UserMessage é o texto seguro para a tela: sem caminho de arquivo, sem saída
// de processo, sem nome de binário.
func (e *Error) UserMessage() string { return e.Kind.Error() }

// SafeExternalURL devolve a URL só quando é segura para virar um href. Links de
// canal vêm da plataforma, ou seja, de fora: sem esta checagem um valor como
// `javascript:...` ou `data:...` viraria execução no navegador de quem clica.
func SafeExternalURL(bruta string) string {
	bruta = strings.TrimSpace(bruta)
	if bruta == "" || len(bruta) > maxURLLength {
		return ""
	}

	parsed, err := url.Parse(bruta)
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return ""
	}
	if parsed.Host == "" {
		return ""
	}

	// Credenciais embutidas não têm uso legítimo em um link de perfil.
	parsed.User = nil
	return parsed.String()
}

func wrap(kind error, detail string) error {
	return &Error{Kind: kind, Detail: detail}
}

// UserMessage extrai a mensagem apresentável de qualquer erro. Erros que não
// vieram do domínio recebem um texto genérico, porque podem carregar detalhe
// técnico que não deve chegar ao usuário.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.UserMessage()
	}
	for _, conhecido := range []error{
		ErrInvalidURL, ErrUnsupportedPlatform, ErrContentUnavailable, ErrContentPrivate,
		ErrAuthRequired, ErrGeoBlocked, ErrLiveContent, ErrFormatUnavailable,
		ErrRateLimited, ErrTimeout, ErrCanceled, ErrProviderUnavailable, ErrDownloadFailed,
	} {
		if errors.Is(err, conhecido) {
			return conhecido.Error()
		}
	}
	return ErrDownloadFailed.Error()
}
