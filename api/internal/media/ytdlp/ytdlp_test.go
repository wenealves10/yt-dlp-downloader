package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

func TestInterpretarProgresso(t *testing.T) {
	progresso, faixa, status, ok := interpretarProgresso("downloading|52428800|104857600|NA|8388608|12|137")
	require.True(t, ok)
	require.InDelta(t, 50.0, progresso.Percent, 0.01)
	require.Equal(t, int64(52428800), progresso.DownloadedBytes)
	require.Equal(t, int64(104857600), progresso.TotalBytes)
	require.Equal(t, int64(8388608), progresso.SpeedBPS)
	require.Equal(t, 12, progresso.ETASeconds)
	require.Equal(t, "137", faixa)
	require.Equal(t, "downloading", status)
}

func TestInterpretarProgressoUsaEstimativaQuandoNaoHaTotalExato(t *testing.T) {
	progresso, _, _, ok := interpretarProgresso("downloading|50|NA|200|1000|4|137")
	require.True(t, ok)
	require.Equal(t, int64(200), progresso.TotalBytes)
	require.InDelta(t, 25.0, progresso.Percent, 0.01)
}

func TestInterpretarProgressoIgnoraFaixaDesconhecida(t *testing.T) {
	// "NA" é o "não sei" do yt-dlp. Tratá-lo como id faria cada amostra parecer
	// uma faixa nova e zeraria a contagem a cada linha.
	_, faixa, _, ok := interpretarProgresso("downloading|50|100|NA|1000|4|NA")
	require.True(t, ok)
	require.Empty(t, faixa)
}

func TestInterpretarProgressoToleraCamposAusentes(t *testing.T) {
	// Nem toda plataforma informa tamanho, velocidade ou ETA. A linha sem o id
	// do formato também é aceita: é o formato antigo do template.
	progresso, faixa, _, ok := interpretarProgresso("downloading|1024|NA|NA|None|NA")
	require.True(t, ok)
	require.Equal(t, int64(1024), progresso.DownloadedBytes)
	require.Zero(t, progresso.TotalBytes)
	require.Zero(t, progresso.Percent)
	require.Empty(t, faixa)

	_, _, _, ok = interpretarProgresso("linha inesperada")
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

// binarioFalso cria um executável que só imprime uma versão, para o health
// check ter algo real para executar sem depender do que está instalado na
// máquina que roda os testes.
func binarioFalso(t *testing.T, nome, saida string) string {
	t.Helper()

	caminho := filepath.Join(t.TempDir(), nome)
	script := "#!/bin/sh\necho \"" + saida + "\"\n"
	if err := os.WriteFile(caminho, []byte(script), 0o700); err != nil {
		t.Fatalf("não foi possível criar o binário falso: %v", err)
	}
	return caminho
}

// Sem ffmpeg, o processo que só resolve metadados continua saudável: ele nunca
// junta faixas. Cobrar a dependência ali pintava de vermelho um container que
// nunca a executou — que era exatamente o defeito do painel.
func TestHealthResolverIgnoraFFmpegAusente(t *testing.T) {
	provider := New(Config{
		Binary:       binarioFalso(t, "yt-dlp", "2025.09.01"),
		FFmpegBinary: filepath.Join(t.TempDir(), "ffmpeg-que-nao-existe"),
		Role:         media.RoleResolver,
	})

	saude := provider.Health(context.Background())

	if !saude.Available {
		t.Fatalf("resolver deveria estar disponível sem ffmpeg, detalhe: %q", saude.Detail)
	}
	if saude.Role != media.RoleResolver {
		t.Fatalf("papel esperado %q, obtido %q", media.RoleResolver, saude.Role)
	}

	ffmpeg, ok := ferramenta(saude.Tools, "ffmpeg")
	if !ok {
		t.Fatal("ffmpeg deveria aparecer na lista mesmo sem ser exigido")
	}
	if ffmpeg.Required {
		t.Error("ffmpeg não deveria ser obrigatório para o resolver")
	}
	if ffmpeg.Usage != media.UsageUnused {
		t.Errorf("o resolver nem executa ffmpeg: uso esperado %q, obtido %q", media.UsageUnused, ffmpeg.Usage)
	}
	if ffmpeg.Available {
		t.Error("ffmpeg inexistente não deveria ser reportado como disponível")
	}
}

// O deno é executado nas DUAS etapas — quem só lê metadados também precisa dele
// para uma requisição autenticada passar. Marcá-lo como não usado foi o que fez
// o painel dizer que ele não servia para nada ali.
func TestHealthDenoEhOpcionalNosDoisPapeis(t *testing.T) {
	for _, papel := range []media.Role{media.RoleResolver, media.RoleDownloader} {
		provider := New(Config{
			Binary:       binarioFalso(t, "yt-dlp", "2025.09.01"),
			FFmpegBinary: binarioFalso(t, "ffmpeg", "ffmpeg version 5.1.9"),
			Role:         papel,
		})

		deno, ok := ferramenta(provider.Health(context.Background()).Tools, "deno")
		if !ok {
			t.Fatalf("[%s] deno deveria aparecer na lista", papel)
		}
		if deno.Usage != media.UsageOptional {
			t.Errorf("[%s] uso esperado %q, obtido %q", papel, media.UsageOptional, deno.Usage)
		}
		if deno.Required {
			t.Errorf("[%s] a ausência do deno degrada, mas não derruba o provider", papel)
		}
		if deno.Note == "" {
			t.Errorf("[%s] o painel precisa dizer o que a ausência custa", papel)
		}
	}
}

// No processo que baixa, a ausência do ffmpeg é falha de verdade: sem ele o
// yt-dlp entrega faixa solta em vez de vídeo com áudio.
func TestHealthDownloaderExigeFFmpeg(t *testing.T) {
	provider := New(Config{
		Binary:       binarioFalso(t, "yt-dlp", "2025.09.01"),
		FFmpegBinary: filepath.Join(t.TempDir(), "ffmpeg-que-nao-existe"),
		Role:         media.RoleDownloader,
	})

	saude := provider.Health(context.Background())

	if saude.Available {
		t.Fatal("downloader sem ffmpeg deveria estar indisponível")
	}
	if saude.Detail == "" {
		t.Error("a falha deveria vir com um detalhe explicando o motivo")
	}

	ffmpeg, ok := ferramenta(saude.Tools, "ffmpeg")
	if !ok {
		t.Fatal("ffmpeg deveria aparecer na lista")
	}
	if !ffmpeg.Required {
		t.Error("ffmpeg deveria ser obrigatório para o downloader")
	}
	if ffmpeg.Usage != media.UsageRequired {
		t.Errorf("uso esperado %q, obtido %q", media.UsageRequired, ffmpeg.Usage)
	}
}

// O papel padrão é o exigente: um provider montado sem Role explícita erra
// para o lado de reportar uma falha a mais, nunca uma a menos.
func TestHealthPapelPadraoEhDownloader(t *testing.T) {
	provider := New(Config{
		Binary:       binarioFalso(t, "yt-dlp", "2025.09.01"),
		FFmpegBinary: filepath.Join(t.TempDir(), "ffmpeg-que-nao-existe"),
	})

	saude := provider.Health(context.Background())

	if saude.Role != media.RoleDownloader {
		t.Fatalf("papel padrão esperado %q, obtido %q", media.RoleDownloader, saude.Role)
	}
	if saude.Available {
		t.Error("o padrão deveria exigir ffmpeg e reportar indisponível")
	}
}

func ferramenta(lista []media.Tool, nome string) (media.Tool, bool) {
	for _, item := range lista {
		if item.Name == nome {
			return item, true
		}
	}
	return media.Tool{}, false
}
