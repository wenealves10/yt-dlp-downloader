package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// saidaJSON é a fatia do --dump-single-json que nos interessa. O yt-dlp devolve
// dezenas de campos; declarar só estes deixa claro do que dependemos.
type saidaJSON struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Thumbnail   string  `json:"thumbnail"`
	Duration    float64 `json:"duration"`
	Uploader    string  `json:"uploader"`
	UploaderID  string  `json:"uploader_id"`
	UploaderURL string  `json:"uploader_url"`
	Channel     string  `json:"channel"`
	ChannelURL  string  `json:"channel_url"`
	// Nem toda plataforma informa estes; ausentes chegam como zero.
	ChannelFollowers int64  `json:"channel_follower_count"`
	ViewCount        int64  `json:"view_count"`
	LikeCount        int64  `json:"like_count"`
	UploadDate       string `json:"upload_date"`
	WebpageURL       string `json:"webpage_url"`
	Extractor        string `json:"extractor_key"`
	IsLive           bool   `json:"is_live"`
	WasLive          bool   `json:"was_live"`
	LiveStatus       string `json:"live_status"`
	// Type distingue vídeo de playlist. Playlists não são baixadas: um link de
	// playlist viraria dezenas de arquivos sem o usuário pedir.
	Type    string        `json:"_type"`
	Formats []formatoJSON `json:"formats"`
}

type formatoJSON struct {
	FormatID       string  `json:"format_id"`
	FormatNote     string  `json:"format_note"`
	Ext            string  `json:"ext"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FPS            float64 `json:"fps"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	TBR            float64 `json:"tbr"`
	ABR            float64 `json:"abr"`
	Protocol       string  `json:"protocol"`
}

func (f formatoJSON) temVideo() bool {
	return f.VCodec != "" && f.VCodec != "none"
}

func (f formatoJSON) temAudio() bool {
	return f.ACodec != "" && f.ACodec != "none"
}

// Metadata resolve o conteúdo sem baixá-lo.
func (p *Provider) Metadata(ctx context.Context, parsed *url.URL, opts media.MetadataOptions) (*media.Metadata, error) {
	args := p.argsBase()
	args = append(args,
		"--dump-single-json",
		"--no-playlist",
		"--skip-download",
	)

	// Vimeo recusa a própria leitura de metadados sem sessão ("the web client
	// only works when logged-in"); resolver sem os cookies falharia antes de o
	// usuário chegar a escolher a qualidade.
	if opts.CookieFile != "" {
		args = append(args, "--cookies", opts.CookieFile)
	}

	args = append(args, "--", parsed.String())

	var bruto strings.Builder
	stderr, err := p.runner.executar(ctx, args, func(linha string) {
		bruto.WriteString(linha)
	})
	if err != nil {
		return nil, classificar(err, stderr)
	}

	var saida saidaJSON
	if err := json.Unmarshal([]byte(bruto.String()), &saida); err != nil {
		return nil, &media.Error{
			Kind:   media.ErrDownloadFailed,
			Detail: fmt.Sprintf("resposta de metadados ilegível: %v", err),
		}
	}

	if saida.Type == "playlist" {
		return nil, &media.Error{
			Kind:   media.ErrUnsupportedPlatform,
			Detail: "a URL aponta para uma playlist, não para um vídeo",
		}
	}

	// Transmissão em andamento não tem fim: o download nunca terminaria.
	if saida.IsLive || saida.LiveStatus == "is_live" || saida.LiveStatus == "is_upcoming" {
		return nil, &media.Error{Kind: media.ErrLiveContent, Detail: saida.LiveStatus}
	}

	uploader := saida.Uploader
	if uploader == "" {
		uploader = saida.Channel
	}

	// uploader_url aponta para o perfil (@usuario) e channel_url para o id
	// interno; o primeiro é o que a pessoa reconhece.
	uploaderURL := saida.UploaderURL
	if uploaderURL == "" {
		uploaderURL = saida.ChannelURL
	}

	return &media.Metadata{
		ContentID:   saida.ID,
		Title:       strings.TrimSpace(saida.Title),
		Description: truncar(saida.Description, 2000),
		Thumbnail:   media.SafeExternalURL(saida.Thumbnail),
		Uploader:    uploader,
		UploaderID:  strings.TrimPrefix(saida.UploaderID, "@"),
		// Vem da plataforma e vira href na tela: só sai daqui se for http(s).
		UploaderURL:   media.SafeExternalURL(uploaderURL),
		FollowerCount: saida.ChannelFollowers,
		ViewCount:     saida.ViewCount,
		LikeCount:     saida.LikeCount,
		UploadDate:    formatarData(saida.UploadDate),
		WebpageURL:    media.SafeExternalURL(saida.WebpageURL),
		Duration:      int(saida.Duration),
		IsLive:        saida.IsLive,
		Formats:       normalizarFormatos(saida.Formats),
	}, nil
}

// segmentado informa se o formato é entregue em fragmentos (HLS, DASH) em vez
// de um arquivo único.
//
// Isto NÃO desqualifica o formato: o yt-dlp baixa os fragmentos e os remuxa em
// um MP4 normal. Serve só como critério de desempate — entre dois formatos da
// mesma altura, o arquivo único é preferível porque informa o tamanho exato e
// dispensa a remuxagem.
func segmentado(f formatoJSON) bool {
	switch f.Protocol {
	case "m3u8", "m3u8_native", "http_dash_segments":
		return true
	}
	return false
}

// melhorQue decide qual dos dois formatos da mesma altura fica na lista.
func melhorQue(candidato, atual formatoJSON) bool {
	// Arquivo único ganha do fragmentado: tamanho exato e sem remuxagem.
	if segmentado(candidato) != segmentado(atual) {
		return !segmentado(candidato)
	}
	if candidato.TBR != atual.TBR {
		return candidato.TBR > atual.TBR
	}
	// Empatando, o que já traz áudio junto evita a etapa de merge.
	return candidato.temAudio() && !atual.temAudio()
}

// normalizarFormatos traduz o vocabulário do yt-dlp para o do domínio e reduz a
// lista ao que faz sentido oferecer.
//
// O yt-dlp devolve trinta ou mais formatos para um vídeo do YouTube, muitos
// deles variações do mesmo que só confundiriam quem escolhe. Ficamos com a
// melhor opção por altura de vídeo, mais as de áudio.
//
// Formatos segmentados (HLS/DASH) CONTAM. Descartá-los, como esta função fazia,
// zerava a lista inteira em Vimeo, Dailymotion, Pinterest, Reddit e X — todos
// servem só HLS. Sem formato nenhum, o download caía num seletor genérico e
// terminava em "o formato escolhido não está disponível", que era o erro
// visível para uma causa que não tinha nada a ver com a escolha do usuário.
func normalizarFormatos(brutos []formatoJSON) []media.Format {
	melhorPorAltura := map[int]formatoJSON{}
	var melhorAudio *formatoJSON

	for _, bruto := range brutos {
		if bruto.FormatID == "" {
			continue
		}
		// Storyboards vêm como mhtml sem vídeo nem áudio; o switch abaixo já os
		// deixa de fora, mas descartar aqui evita percorrê-los.
		if bruto.Protocol == "mhtml" {
			continue
		}

		switch {
		case bruto.temVideo() && bruto.Height > 0:
			atual, existe := melhorPorAltura[bruto.Height]
			if !existe || melhorQue(bruto, atual) {
				melhorPorAltura[bruto.Height] = bruto
			}
		case !bruto.temVideo() && bruto.temAudio():
			if melhorAudio == nil || bruto.ABR > melhorAudio.ABR {
				copia := bruto
				melhorAudio = &copia
			}
		}
	}

	formatos := make([]media.Format, 0, len(melhorPorAltura)+1)
	for altura, bruto := range melhorPorAltura {
		formatos = append(formatos, media.Format{
			ID:              bruto.FormatID,
			Label:           fmt.Sprintf("%dp", altura),
			Kind:            media.KindVideo,
			Ext:             bruto.Ext,
			Width:           bruto.Width,
			Height:          altura,
			FPS:             int(bruto.FPS),
			VideoCodec:      codecCurto(bruto.VCodec),
			AudioCodec:      codecCurto(bruto.ACodec),
			SizeBytes:       tamanho(bruto),
			SizeApproximate: bruto.Filesize == 0 && bruto.FilesizeApprox > 0,
		})
	}

	// Maior resolução primeiro: é a escolha mais comum.
	sort.Slice(formatos, func(i, j int) bool {
		return formatos[i].Height > formatos[j].Height
	})

	if melhorAudio != nil {
		rotulo := "Áudio"
		if melhorAudio.ABR > 0 {
			rotulo = fmt.Sprintf("Áudio %.0f kbps", melhorAudio.ABR)
		}
		formatos = append(formatos, media.Format{
			ID:              melhorAudio.FormatID,
			Label:           rotulo,
			Kind:            media.KindAudio,
			Ext:             melhorAudio.Ext,
			AudioCodec:      codecCurto(melhorAudio.ACodec),
			SizeBytes:       tamanho(*melhorAudio),
			SizeApproximate: melhorAudio.Filesize == 0 && melhorAudio.FilesizeApprox > 0,
		})
	}

	return formatos
}

func tamanho(f formatoJSON) int64 {
	if f.Filesize > 0 {
		return f.Filesize
	}
	return f.FilesizeApprox
}

// codecCurto corta a variante longa ("avc1.640028" vira "avc1"), que é ruído na
// tela.
func codecCurto(codec string) string {
	if codec == "" || codec == "none" {
		return ""
	}
	if ponto := strings.IndexByte(codec, '.'); ponto > 0 {
		return codec[:ponto]
	}
	return codec
}

// formatarData converte o "20050424" do yt-dlp para ISO, que é o que a tela
// sabe interpretar. Valor fora do formato é descartado em vez de exibido cru.
func formatarData(bruta string) string {
	if len(bruta) != 8 {
		return ""
	}
	if _, err := time.Parse("20060102", bruta); err != nil {
		return ""
	}
	return bruta[:4] + "-" + bruta[4:6] + "-" + bruta[6:]
}

func truncar(texto string, limite int) string {
	texto = strings.TrimSpace(texto)
	if len(texto) <= limite {
		return texto
	}
	return texto[:limite] + "…"
}
