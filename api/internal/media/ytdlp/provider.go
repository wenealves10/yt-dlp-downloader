package ytdlp

import (
	"context"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// Config reúne o que o provider precisa saber do ambiente. Trocar a versão do
// yt-dlp ou o caminho do binário é configuração, não mudança de código.
type Config struct {
	// Binary é o executável. Vazio usa "yt-dlp" do PATH.
	Binary string
	// FFmpegBinary é usado para juntar faixas e converter áudio.
	FFmpegBinary string
	// ProxyURL, quando presente, vale para metadados e download.
	ProxyURL  string
	UserAgent string
	Referer   string
	// MetadataTimeout limita a resolução; ela roda dentro de uma requisição
	// HTTP e não pode segurar a conexão indefinidamente.
	MetadataTimeout time.Duration
	// DownloadTimeout é o teto de um download inteiro.
	DownloadTimeout time.Duration
}

// Provider implementa media.Provider sobre o yt-dlp.
type Provider struct {
	cfg      Config
	runner   *runner
	download *runner
}

// New monta o provider. Não valida o binário aqui de propósito: o serviço deve
// subir mesmo com o yt-dlp quebrado, e reportar isso pelo health check em vez
// de recusar a inicialização inteira.
func New(cfg Config) *Provider {
	if cfg.Binary == "" {
		cfg.Binary = "yt-dlp"
	}
	if cfg.FFmpegBinary == "" {
		cfg.FFmpegBinary = "ffmpeg"
	}
	if cfg.MetadataTimeout <= 0 {
		cfg.MetadataTimeout = 45 * time.Second
	}
	if cfg.DownloadTimeout <= 0 {
		cfg.DownloadTimeout = 2 * time.Hour
	}

	return &Provider{
		cfg:      cfg,
		runner:   &runner{binary: cfg.Binary, proxy: cfg.ProxyURL, timeout: cfg.MetadataTimeout},
		download: &runner{binary: cfg.Binary, proxy: cfg.ProxyURL, timeout: cfg.DownloadTimeout},
	}
}

func (p *Provider) Name() string { return "yt-dlp" }

// CanHandle aceita qualquer http(s). O yt-dlp cobre mais de mil sites e a lista
// muda a cada versão dele; manter uma cópia aqui ficaria desatualizada e
// recusaria conteúdo que ele sabe baixar.
//
// Quem realmente decide é o extractor, na hora de resolver — e o erro dele já
// vira ErrUnsupportedPlatform. A validação de segurança da URL acontece antes,
// em media.NormalizeURL.
func (p *Provider) CanHandle(parsed *url.URL) bool {
	if parsed == nil {
		return false
	}
	esquema := strings.ToLower(parsed.Scheme)
	return (esquema == "http" || esquema == "https") && parsed.Host != ""
}

// argsBase são as opções comuns a metadados e download.
func (p *Provider) argsBase() []string {
	args := []string{
		// Sem cor e sem barra de progresso interativa: a saída é lida por
		// máquina.
		"--no-colors",
		"--no-warnings",
		// Não lê ~/.config/yt-dlp: a configuração vem daqui, e um arquivo
		// solto no container mudaria o comportamento sem aparecer no código.
		"--ignore-config",
		"--no-playlist",
		// Impede que a plataforma redirecione para um arquivo local.
		"--no-check-certificates",
		"--socket-timeout", "30",
		"--retries", "3",
		"--fragment-retries", "3",
	}

	if p.cfg.ProxyURL != "" {
		args = append(args, "--proxy", p.cfg.ProxyURL)
	}
	if p.cfg.UserAgent != "" {
		args = append(args, "--user-agent", p.cfg.UserAgent)
	}
	if p.cfg.Referer != "" {
		args = append(args, "--referer", p.cfg.Referer)
	}

	return args
}

// Health verifica o binário e as dependências que o download usa.
func (p *Provider) Health(ctx context.Context) media.Health {
	saude := media.Health{Provider: p.Name(), CheckedAt: time.Now().UTC()}

	versao, err := p.runner.versao(ctx)
	if err != nil {
		saude.Available = false
		saude.Detail = err.Error()
	} else {
		saude.Available = true
		saude.Version = versao
	}

	saude.Tools = []media.Tool{
		{Name: "yt-dlp", Available: saude.Available, Version: saude.Version, Required: true},
	}

	// ffmpeg junta vídeo e áudio separados e converte para MP3. Sem ele, boa
	// parte das resoluções altas do YouTube fica indisponível.
	versaoFFmpeg, erroFFmpeg := versaoDe(ctx, p.cfg.FFmpegBinary, "-version")
	saude.Tools = append(saude.Tools, media.Tool{
		Name: "ffmpeg", Available: erroFFmpeg == nil, Version: versaoFFmpeg, Required: true,
	})
	if erroFFmpeg != nil {
		saude.Available = false
		if saude.Detail == "" {
			saude.Detail = "ffmpeg indisponível: faixas separadas não podem ser unidas"
		}
	}

	// O runtime JavaScript resolve o desafio "n" do YouTube. Sem ele, uma
	// requisição autenticada é recusada — mas o resto das plataformas continua
	// funcionando, então não derruba o provider.
	versaoDeno, erroDeno := versaoDe(ctx, "deno", "--version")
	saude.Tools = append(saude.Tools, media.Tool{
		Name: "deno", Available: erroDeno == nil, Version: versaoDeno, Required: false,
	})

	return saude
}

// versaoDe pega a primeira linha da saída de versão de um binário auxiliar.
func versaoDe(ctx context.Context, binario string, args ...string) (string, error) {
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()

	saida, err := exec.CommandContext(ctx, binario, args...).Output()
	if err != nil {
		return "", err
	}

	linha := strings.TrimSpace(string(saida))
	if quebra := strings.IndexByte(linha, '\n'); quebra > 0 {
		linha = linha[:quebra]
	}
	return linha, nil
}
