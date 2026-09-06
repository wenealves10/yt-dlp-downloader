// Package providers monta o conjunto de providers de download a partir da
// configuração. É a camada de ligação: `media` define o contrato e `ytdlp` o
// implementa, e nenhum dos dois pode conhecer o outro sem criar um ciclo.
//
// É também o ÚNICO lugar que decide quais providers existem. Somar um provider
// específico de plataforma no futuro é acrescentar uma linha aqui — handlers,
// jobs, banco e frontend seguem intactos.
package providers

import (
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/media/ytdlp"
)

// Registry monta o registry na ordem de preferência: o primeiro que aceitar a
// URL atende, e os seguintes servem de fallback quando ele falha por
// indisponibilidade.
func Registry(cfg configs.Config, role media.Role) *media.Registry {
	proxy := ""
	if cfg.ProxyEnabled {
		proxy = cfg.ProxyURL
	}

	ytdlpProvider := ytdlp.New(ytdlp.Config{
		Binary:       cfg.YtDlpBinary,
		FFmpegBinary: cfg.FFmpegBinary,
		ProxyURL:     proxy,
		// Independe de PROXY_ENABLED: o proxy de resolução é uma decisão
		// separada, com custo e finalidade próprios. Ligar o proxy geral
		// mandaria o vídeo inteiro por ele, que é o oposto do que se quer.
		ResolveProxyURL: cfg.ResolveProxyURL,
		UserAgent:       cfg.YoutubeDLUserAgent,
		Referer:         cfg.YoutubeDLReferer,
		MetadataTimeout: duracaoOu(cfg.MediaMetadataTimeout, 45*time.Second),
		DownloadTimeout: duracaoOu(cfg.MediaDownloadTimeout, 2*time.Hour),
		Role:            role,
	})

	// Ordem futura, quando houver providers dedicados:
	//   return media.NewRegistry(instagramProvider, tiktokProvider, ytdlpProvider)
	// O yt-dlp fica por último e vira o fallback geral.
	return media.NewRegistry(ytdlpProvider)
}

func duracaoOu(valor, padrao time.Duration) time.Duration {
	if valor <= 0 {
		return padrao
	}
	return valor
}
