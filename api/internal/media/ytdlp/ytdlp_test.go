package ytdlp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

func TestInterpretarProgresso(t *testing.T) {
	progresso, ok := interpretarProgresso("downloading|52428800|104857600|NA|8388608|12")
	require.True(t, ok)
	require.InDelta(t, 50.0, progresso.Percent, 0.01)
	require.Equal(t, int64(52428800), progresso.DownloadedBytes)
	require.Equal(t, int64(104857600), progresso.TotalBytes)
	require.Equal(t, int64(8388608), progresso.SpeedBPS)
	require.Equal(t, 12, progresso.ETASeconds)
	require.False(t, progresso.Postprocess)
}

func TestInterpretarProgressoUsaEstimativaQuandoNaoHaTotalExato(t *testing.T) {
	progresso, ok := interpretarProgresso("downloading|50|NA|200|1000|4")
	require.True(t, ok)
	require.Equal(t, int64(200), progresso.TotalBytes)
	require.InDelta(t, 25.0, progresso.Percent, 0.01)
}

func TestInterpretarProgressoMarcaPosProcessamento(t *testing.T) {
	// "finished" no download bruto significa que começou o merge/conversão. Sem
	// marcar, a barra fica em 100% parecendo travada.
	progresso, ok := interpretarProgresso("finished|104857600|104857600|NA|NA|NA")
	require.True(t, ok)
	require.True(t, progresso.Postprocess)
	require.InDelta(t, 100.0, progresso.Percent, 0.01)
}

func TestInterpretarProgressoToleraCamposAusentes(t *testing.T) {
	// Nem toda plataforma informa tamanho, velocidade ou ETA.
	progresso, ok := interpretarProgresso("downloading|1024|NA|NA|None|NA")
	require.True(t, ok)
	require.Equal(t, int64(1024), progresso.DownloadedBytes)
	require.Zero(t, progresso.TotalBytes)
	require.Zero(t, progresso.Percent)

	_, ok = interpretarProgresso("linha inesperada")
	require.False(t, ok)
}

func TestNormalizarFormatosReduzOExcesso(t *testing.T) {
	// O yt-dlp devolve dezenas de variações do mesmo formato; oferecer todas
	// só atrapalharia quem escolhe.
	brutos := []formatoJSON{
		{FormatID: "137", Ext: "mp4", Height: 1080, Width: 1920, VCodec: "avc1.640028", ACodec: "none", TBR: 4000, Filesize: 100},
		{FormatID: "248", Ext: "webm", Height: 1080, Width: 1920, VCodec: "vp9", ACodec: "none", TBR: 3000},
		{FormatID: "136", Ext: "mp4", Height: 720, Width: 1280, VCodec: "avc1", ACodec: "none", TBR: 2000},
		{FormatID: "140", Ext: "m4a", VCodec: "none", ACodec: "mp4a.40.2", ABR: 128, Filesize: 50},
		{FormatID: "251", Ext: "webm", VCodec: "none", ACodec: "opus", ABR: 160},
		// Streaming não produz arquivo final utilizável aqui.
		{FormatID: "hls-1", Ext: "mp4", Height: 480, VCodec: "avc1", Protocol: "m3u8_native"},
	}

	formatos := normalizarFormatos(brutos)

	require.Len(t, formatos, 3, "1080p, 720p e áudio")
	// Maior resolução primeiro: é a escolha mais comum.
	require.Equal(t, "1080p", formatos[0].Label)
	require.Equal(t, "137", formatos[0].ID, "entre dois 1080p vence o de maior taxa de bits")
	require.Equal(t, "720p", formatos[1].Label)
	require.Equal(t, media.KindAudio, formatos[2].Kind)
	require.Equal(t, "251", formatos[2].ID, "vence o áudio de maior bitrate")

	for _, formato := range formatos {
		require.NotContains(t, formato.ID, "hls")
	}
	require.Equal(t, "avc1", formatos[0].VideoCodec, "o codec longo é encurtado")
}

func TestArgsArquivoNuncaMontaComandoPorConcatenacao(t *testing.T) {
	provider := New(Config{Binary: "yt-dlp", ProxyURL: "http://proxy:7000"})

	args := provider.argsArquivo(media.Request{
		URL:      "https://youtu.be/abc",
		Kind:     media.KindVideo,
		FormatID: "137",
		Filename: "video_abc",
	}, "/tmp/saida")

	// O "--" precisa vir logo antes da URL: sem ele, uma URL começando com
	// hífen viraria opção do processo.
	require.Equal(t, "https://youtu.be/abc", args[len(args)-1])
	require.Equal(t, "--", args[len(args)-2])

	// A configuração do container não pode alterar o comportamento por fora.
	require.Contains(t, args, "--ignore-config")
	require.Contains(t, args, "--no-playlist")
}

func TestDownloadRecusaEntradaForaDoVocabulario(t *testing.T) {
	provider := New(Config{})

	// Um format_id vindo do cliente não pode virar outra opção de comando.
	_, err := provider.Download(context.Background(), media.Request{
		URL: "https://youtu.be/a", Filename: "ok", FormatID: "--exec=rm -rf /",
	}, nil)
	require.ErrorIs(t, err, media.ErrFormatUnavailable)

	// Nome de arquivo com travessia de diretório.
	for _, nome := range []string{"../escapa", "a/b", "", "com espaço", "arquivo;rm"} {
		_, err := provider.Download(context.Background(), media.Request{
			URL: "https://youtu.be/a", Filename: nome,
		}, nil)
		require.ErrorIs(t, err, media.ErrDownloadFailed, "nome: %q", nome)
	}
}

func TestClassificarTraduzOsErrosConhecidos(t *testing.T) {
	casos := []struct {
		stderr string
		tipo   error
	}{
		{"ERROR: [youtube] abc: Private video. Sign in if you've been granted access", media.ErrContentPrivate},
		{"ERROR: [youtube] abc: Video unavailable", media.ErrContentUnavailable},
		{"ERROR: [youtube] abc: Sign in to confirm you're not a bot", media.ErrAuthRequired},
		{"ERROR: [generic] The uploader has not made this video available in your country", media.ErrGeoBlocked},
		{"ERROR: Requested format is not available", media.ErrFormatUnavailable},
		{"ERROR: HTTP Error 429: Too Many Requests", media.ErrRateLimited},
		{"ERROR: Unsupported URL: https://exemplo.com/x", media.ErrUnsupportedPlatform},
		{"ERROR: [twitch] 123: The stream is live", media.ErrLiveContent},
	}

	for _, caso := range casos {
		err := classificar(errFalso{}, caso.stderr)
		require.ErrorIs(t, err, caso.tipo, caso.stderr)
		// O texto para o usuário nunca carrega a saída bruta.
		require.NotContains(t, media.UserMessage(err), "ERROR:")
	}
}

func TestResumirStderrDescartaAvisos(t *testing.T) {
	stderr := "WARNING: [youtube] No supported JavaScript runtime could be found\n" +
		"WARNING: n challenge solving failed\n" +
		"ERROR: [youtube] abc: Video unavailable\n"

	resumo := resumirStderr(stderr)
	require.Contains(t, resumo, "Video unavailable")
	require.NotContains(t, resumo, "WARNING")
}

type errFalso struct{}

func (errFalso) Error() string { return "exit status 1" }
