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
	ProxyURL string
	// ResolveProxyURL vale APENAS para a extração de metadados, e apenas nas
	// plataformas que recusam IP de datacenter. Os bytes do vídeo nunca passam
	// por ele: é o que permite usar um proxy residencial pago por volume sem
	// queimar a cota — a extração custa dezenas de KB, o vídeo dezenas de MB.
	ResolveProxyURL string
	UserAgent       string
	Referer         string
	// MetadataTimeout limita a resolução; ela roda dentro de uma requisição
	// HTTP e não pode segurar a conexão indefinidamente.
	MetadataTimeout time.Duration
	// DownloadTimeout é o teto de um download inteiro.
	DownloadTimeout time.Duration
	// Role diz o que este processo faz com o provider. O padrão é downloader,
	// que é o exigente: errar para o lado estrito reporta uma falha a mais, e
	// não uma a menos.
	Role media.Role
}

// Provider implementa media.Provider sobre o yt-dlp.
type Provider struct {
	cfg      Config
	runner   *runner
	download *runner
	// impersonacao é sondada sob demanda e guardada aqui: é caro descobrir e
	// não muda enquanto o processo viver.
	impersonacao deteccaoImpersonacao
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
	if cfg.Role == "" {
		cfg.Role = media.RoleDownloader
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
// argsBase monta os argumentos comuns. A plataforma entra porque duas decisões
// dependem dela: imitar ou não um navegador, e enviar ou não o nosso
// User-Agent — as duas são incompatíveis entre si.
// fase distingue os dois tipos de requisição que o yt-dlp faz. Elas têm ordens
// de grandeza diferentes de tráfego, e por isso podem sair por caminhos
// diferentes.
type fase int

const (
	// faseExtracao pede metadados: algumas dezenas de KB.
	faseExtracao fase = iota
	// faseMidia baixa os bytes do conteúdo: de MB a GB.
	faseMidia
)

// proxyDa escolhe por onde a requisição sai.
//
// Só a extração pode usar o proxy de resolução, e só nas plataformas que
// recusam IP de datacenter. Mandar a mídia por ele queimaria a cota do proxy
// residencial em poucos vídeos, que é exatamente o que a separação evita.
func (p *Provider) proxyDa(plataforma media.Platform, etapa fase) string {
	if etapa == faseExtracao && p.cfg.ResolveProxyURL != "" &&
		plataformasQueRecusamDatacenter[plataforma] {
		return p.cfg.ResolveProxyURL
	}
	return p.cfg.ProxyURL
}

func (p *Provider) argsBase(plataforma media.Platform, etapa fase) []string {
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

	if proxy := p.proxyDa(plataforma, etapa); proxy != "" {
		args = append(args, "--proxy", proxy)
	}

	args = append(args, p.argsImpersonacao(plataforma)...)

	// Sob impersonação o User-Agent vem do próprio alvo imitado; sobrescrevê-lo
	// deixaria o handshake e o cabeçalho contando histórias diferentes.
	if p.cfg.UserAgent != "" && !p.usaImpersonacao(plataforma) {
		args = append(args, "--user-agent", p.cfg.UserAgent)
	}
	if p.cfg.Referer != "" {
		args = append(args, "--referer", p.cfg.Referer)
	}

	return args
}

// Health verifica o binário e as dependências que o download usa.
func (p *Provider) Health(ctx context.Context) media.Health {
	saude := media.Health{Provider: p.Name(), Role: p.cfg.Role, CheckedAt: time.Now().UTC()}

	versao, err := p.runner.versao(ctx)
	if err != nil {
		saude.Available = false
		saude.Detail = err.Error()
	} else {
		saude.Available = true
		saude.Version = versao
	}

	saude.Tools = []media.Tool{{
		Name:      "yt-dlp",
		Available: saude.Available,
		Version:   saude.Version,
		Required:  true,
		Usage:     media.UsageRequired,
		Note:      "é o mecanismo em si: fala com a plataforma e lê os formatos",
	}}

	// ffmpeg junta vídeo e áudio separados e converte para MP3 — no processo
	// que BAIXA. Quem só resolve metadados nunca o executa, e exigi-lo ali
	// pintaria de vermelho um processo perfeitamente saudável.
	precisaFFmpeg := p.cfg.Role == media.RoleDownloader
	versaoFFmpeg, erroFFmpeg := versaoDe(ctx, p.cfg.FFmpegBinary, "-version")
	usoFFmpeg, notaFFmpeg := media.UsageUnused, "esta etapa só lê metadados; nada de mídia é aberto ou convertido aqui"
	if precisaFFmpeg {
		usoFFmpeg = media.UsageRequired
		notaFFmpeg = "junta as faixas separadas de vídeo e áudio (acima de 720p) e converte para MP3"
	}
	saude.Tools = append(saude.Tools, media.Tool{
		Name:      "ffmpeg",
		Available: erroFFmpeg == nil,
		Version:   versaoFFmpeg,
		Required:  precisaFFmpeg,
		Usage:     usoFFmpeg,
		Note:      notaFFmpeg,
	})
	if erroFFmpeg != nil && precisaFFmpeg {
		saude.Available = false
		if saude.Detail == "" {
			saude.Detail = "ffmpeg indisponível: faixas separadas não podem ser unidas"
		}
	}

	// Impersonação de navegador. Sem ela, Reddit, X e Pinterest respondem
	// `403: Blocked` mesmo com sessão autenticada, porque o bloqueio é por
	// fingerprint de TLS e acontece antes de qualquer cookie ser olhado.
	temImpersonacao := p.suportaImpersonacao()
	saude.Tools = append(saude.Tools, media.Tool{
		Name:      "impersonação (curl-cffi)",
		Available: temImpersonacao,
		Required:  false,
		Usage:     media.UsageOptional,
		Note:      "imita o handshake de um navegador; sem ela Reddit, X e Pinterest recusam o acesso",
	})

	// O runtime JavaScript resolve o desafio "n" do YouTube, e é executado nas
	// DUAS etapas: quem só lê metadados também precisa dele para uma requisição
	// autenticada passar. Opcional, porém, não é o mesmo que não usado — o
	// painel já confundiu os dois: sem deno o YouTube perde formatos e as
	// requisições com cookies falham, enquanto o resto das plataformas segue
	// funcionando.
	versaoDeno, erroDeno := versaoDe(ctx, "deno", "--version")
	saude.Tools = append(saude.Tools, media.Tool{
		Name:      "deno",
		Available: erroDeno == nil,
		Version:   versaoDeno,
		Required:  false,
		Usage:     media.UsageOptional,
		Note:      "resolve o desafio \"n\" do YouTube; sem ele, requisições com conta autenticada falham",
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
