package integrations

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// MontarDownloadPayload traduz a linha do banco para a visão pública do
// download.
//
// Esta é a ÚNICA função que monta esse objeto, e é de propósito: o corpo do
// webhook e a resposta de `GET /downloads/{id}` têm de ser idênticos. Com duas
// implementações, um campo acrescentado numa delas faria o cliente escrever um
// segundo parser para o mesmo objeto — e a documentação deixaria de descrever
// as duas ao mesmo tempo.
//
// O que NÃO entra aqui é tão importante quanto o que entra: `provider` (o
// mecanismo que baixa) e `error_detail` (o motivo técnico, muitas vezes com
// saída de processo) descrevem a nossa infraestrutura. Eles já não saem para o
// usuário comum, e um sistema cliente não é mais confiável que ele.
func MontarDownloadPayload(download db.Download, baseURL string) DownloadPayload {
	payload := DownloadPayload{
		ID:              download.ID.String(),
		Status:          string(download.Status),
		Title:           download.Title,
		OriginalURL:     download.OriginalUrl,
		Platform:        download.Platform,
		PlatformLabel:   media.Platform(download.Platform).Label(),
		Format:          string(download.Format),
		QualityLabel:    download.QualityLabel.String,
		Uploader:        download.Uploader.String,
		ThumbnailURL:    download.ThumbnailUrl.String,
		DurationSeconds: download.DurationSeconds.Int32,
		FileSizeBytes:   download.FileSizeBytes,
		ErrorMessage:    download.ErrorMessage.String,
		CreatedAt:       download.CreatedAt,
		StartedAt:       instante(download.StartedAt),
		FinishedAt:      instante(download.FinishedAt),
		ExpiresAt:       instante(download.ExpiresAt),
	}

	// O progresso só vai junto enquanto há progresso a relatar. Em um download
	// concluído ele seria ruído, e em um que falhou apontaria para um número
	// que já não descreve nada.
	if download.Status == db.CoreDownloadStatusPROCESSING ||
		download.Status == db.CoreDownloadStatusRETRYING {
		payload.Progress = &ProgressPayload{
			Percent:         percentual(download),
			DownloadedBytes: download.DownloadedBytes,
			TotalBytes:      download.TotalBytes,
			SpeedBPS:        download.SpeedBps,
			ETASeconds:      int(download.EtaSeconds),
		}
	}

	payload.Links = montarLinks(download, baseURL)
	return payload
}

// montarLinks aponta para as rotas da API, nunca para o bucket.
//
// As rotas de arquivo só aparecem quando há arquivo: oferecer `download_url` em
// um download que falhou levaria o cliente a uma chamada que responde 409, e o
// erro pareceria nosso.
func montarLinks(download db.Download, baseURL string) *DownloadLinks {
	if baseURL == "" {
		return nil
	}

	base := baseURL + "/v1/integration/downloads/" + download.ID.String()
	links := &DownloadLinks{Self: base}

	pronto := download.Status == db.CoreDownloadStatusCOMPLETED &&
		download.FileUrl.Valid && download.FileUrl.String != ""
	if pronto {
		links.DownloadURL = base + "/download-url"
		links.File = base + "/file"
	}
	return links
}

// percentual devolve o andamento como número.
//
// A coluna é NUMERIC(5,2) e chega como pgtype.Numeric; converter à mão evita
// tanto o pânico de um valor nulo quanto o custo de arrastar big.Float até a
// resposta JSON.
func percentual(download db.Download) float64 {
	if !download.ProgressPercent.Valid || download.ProgressPercent.Int == nil {
		// Sem o instantâneo gravado, o que se sabe dos bytes ainda vale.
		if download.TotalBytes > 0 {
			return float64(download.DownloadedBytes) / float64(download.TotalBytes) * 100
		}
		return 0
	}

	valor, err := download.ProgressPercent.Float64Value()
	if err != nil || !valor.Valid {
		return 0
	}
	return valor.Float64
}

// instante converte um carimbo opcional em ponteiro.
//
// O ponteiro é o que faz `omitempty` funcionar: um time.Time zerado não é
// omitido pelo encoder, e o cliente receberia "0001-01-01" em vez de nada —
// uma data que parece real e não é.
func instante(valor pgtype.Timestamptz) *time.Time {
	if !valor.Valid {
		return nil
	}
	momento := valor.Time
	return &momento
}
