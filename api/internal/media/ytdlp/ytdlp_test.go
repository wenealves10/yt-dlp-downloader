package ytdlp

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
		// HLS é uma opção legítima: o yt-dlp baixa os fragmentos e remuxa. Este
		// caso já foi tratado como descartável, e era o que zerava a lista
		// inteira nas plataformas que só servem streaming.
		{FormatID: "hls-1", Ext: "mp4", Height: 480, VCodec: "avc1", Protocol: "m3u8_native"},
	}

	formatos := normalizarFormatos(brutos)

	require.Len(t, formatos, 4, "1080p, 720p, 480p e áudio")
	// Maior resolução primeiro: é a escolha mais comum.
	require.Equal(t, "1080p", formatos[0].Label)
	require.Equal(t, "137", formatos[0].ID, "entre dois 1080p vence o de maior taxa de bits")
	require.Equal(t, "720p", formatos[1].Label)
	require.Equal(t, "480p", formatos[2].Label)
	require.Equal(t, media.KindAudio, formatos[3].Kind)
	require.Equal(t, "251", formatos[3].ID, "vence o áudio de maior bitrate")
	require.Equal(t, "avc1", formatos[0].VideoCodec, "o codec longo é encurtado")
}

func TestArgsArquivoNuncaMontaComandoPorConcatenacao(t *testing.T) {
	provider := New(Config{Binary: "yt-dlp", ProxyURL: "http://proxy:7000"})

	args := provider.argsArquivo(media.Request{
		URL:      "https://youtu.be/abc",
		Kind:     media.KindVideo,
		FormatID: "137",
		Filename: "video_abc",
	}, "/tmp/saida", "")

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

// O bug que zerava a lista: Vimeo, Dailymotion, Pinterest, Reddit e X servem
// SÓ HLS. Descartar formatos segmentados deixava o conteúdo sem nenhuma opção,
// e o download terminava em "o formato escolhido não está disponível".
func TestNormalizarFormatosAceitaHLS(t *testing.T) {
	// Saída real do Dailymotion: três formatos, todos m3u8_native, todos já com
	// áudio embutido.
	brutos := []formatoJSON{
		{FormatID: "hls-380", Ext: "mp4", Height: 288, Protocol: "m3u8_native", VCodec: "avc1.42001e", ACodec: "mp4a.40.2", TBR: 380},
		{FormatID: "hls-480", Ext: "mp4", Height: 480, Protocol: "m3u8_native", VCodec: "avc1.64001f", ACodec: "mp4a.40.2", TBR: 480},
		{FormatID: "hls-720", Ext: "mp4", Height: 720, Protocol: "m3u8_native", VCodec: "avc1.64001f", ACodec: "mp4a.40.2", TBR: 720},
	}

	formatos := normalizarFormatos(brutos)

	require.Len(t, formatos, 3, "nenhum formato HLS pode ser descartado")
	require.Equal(t, "hls-720", formatos[0].ID, "a maior resolução vem primeiro")
	require.Equal(t, 720, formatos[0].Height)
}

// DASH também é segmentado e também conta.
func TestNormalizarFormatosAceitaDASH(t *testing.T) {
	brutos := []formatoJSON{
		{FormatID: "dash-6", Ext: "mp4", Height: 1080, Protocol: "http_dash_segments", VCodec: "avc1", ACodec: "none", TBR: 3000},
		{FormatID: "dash-audio", Ext: "m4a", Protocol: "http_dash_segments", VCodec: "none", ACodec: "mp4a", ABR: 128},
	}

	formatos := normalizarFormatos(brutos)

	require.Len(t, formatos, 2)
	require.Equal(t, media.KindVideo, formatos[0].Kind)
	require.Equal(t, media.KindAudio, formatos[1].Kind)
}

// Na mesma altura, o arquivo único ganha do fragmentado: informa o tamanho
// exato e dispensa a remuxagem.
func TestNormalizarFormatosPrefereArquivoUnicoAoSegmentado(t *testing.T) {
	brutos := []formatoJSON{
		{FormatID: "hls-720", Ext: "mp4", Height: 720, Protocol: "m3u8_native", VCodec: "avc1", ACodec: "mp4a", TBR: 5000},
		{FormatID: "http-720", Ext: "mp4", Height: 720, Protocol: "https", VCodec: "avc1", ACodec: "mp4a", TBR: 2000, Filesize: 1000},
	}

	formatos := normalizarFormatos(brutos)

	require.Len(t, formatos, 1)
	require.Equal(t, "http-720", formatos[0].ID,
		"o progressivo vence mesmo com taxa de bits menor")
	require.Equal(t, int64(1000), formatos[0].SizeBytes)
}

// Storyboards não são mídia e não podem virar opção de qualidade.
func TestNormalizarFormatosIgnoraStoryboards(t *testing.T) {
	brutos := []formatoJSON{
		{FormatID: "sb0", Ext: "mhtml", Height: 90, Protocol: "mhtml", VCodec: "none", ACodec: "none"},
		{FormatID: "hls-720", Ext: "mp4", Height: 720, Protocol: "m3u8_native", VCodec: "avc1", ACodec: "mp4a"},
	}

	formatos := normalizarFormatos(brutos)

	require.Len(t, formatos, 1)
	require.Equal(t, "hls-720", formatos[0].ID)
}

// O seletor precisa degradar em vez de desistir: os ids do yt-dlp não são
// estáveis entre duas extrações, e o id que a tela ofereceu pode não existir
// mais quando o worker vai baixar.
func TestSeletorVideoDegradaPorAltura(t *testing.T) {
	seletor := seletorVideo("hls-fastly_skyfire-3609", 720)
	alternativas := strings.Split(seletor, "/")

	require.Equal(t, "hls-fastly_skyfire-3609+bestaudio", alternativas[0],
		"o formato pedido vem primeiro, somado ao áudio")
	require.Equal(t, "hls-fastly_skyfire-3609", alternativas[1],
		"depois ele sozinho, para quando já traz áudio")
	require.Contains(t, seletor, "bestvideo[height<=720]+bestaudio",
		"se o id sumiu, a mesma resolução por outro caminho")
	require.Equal(t, "best", alternativas[len(alternativas)-1],
		"a última alternativa nunca deixa voltar de mãos vazias")
}

func TestSeletorVideoSemAlturaConhecida(t *testing.T) {
	seletor := seletorVideo("137", 0)

	require.Contains(t, seletor, "137+bestaudio")
	require.NotContains(t, seletor, "height<=",
		"sem altura não há como limitar; inventar um número daria a resolução errada")
	require.Contains(t, seletor, "best")
}

func TestSeletorVideoSemFormatoEscolhido(t *testing.T) {
	seletor := seletorVideo("", 0)

	require.Equal(t, "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/bestvideo+bestaudio/best", seletor)
}

func TestSeletorAudioDegrada(t *testing.T) {
	require.Equal(t, "140/bestaudio/best", seletorAudio("140"))
	require.Equal(t, "bestaudio/best", seletorAudio(""))
}

// Bloqueio de IP: a plataforma recusa ESTE servidor, não o conteúdo. Sem estes
// padrões o caso caía no genérico "não foi possível concluir o download", que
// não diz nada a quem opera — e é justamente o que acontece com IP de
// datacenter em Reddit, X e Pinterest.
func TestClassificarReconheceBloqueioDoServidor(t *testing.T) {
	casos := map[string]string{
		"403":     "ERROR: [Reddit] abc: Unable to download webpage: HTTP Error 403: Forbidden",
		"401":     "ERROR: [generic] x: Unable to download webpage: HTTP Error 401: Unauthorized",
		"blocked": "ERROR: [twitter] 1: Your IP address has been blocked",
		"captcha": "ERROR: [pinterest] 1: Please solve the captcha to continue",
		"5xx":     "ERROR: [Reddit] abc: Unable to download webpage: HTTP Error 503: Service Unavailable",
		"negado":  "ERROR: [vimeo] 1: Access denied for this resource",
	}

	for nome, stderr := range casos {
		err := classificar(nil, stderr)
		require.ErrorIs(t, err, media.ErrBlocked, nome)
		require.NotEmpty(t, media.Detail(err), "[%s] o detalhe técnico precisa sobreviver", nome)
	}
}

// Falha de rede antes de qualquer resposta não é problema do conteúdo.
func TestClassificarReconheceFalhaDeRede(t *testing.T) {
	casos := []string{
		"ERROR: unable to download webpage: <urlopen error [Errno 104] Connection reset by peer>",
		"ERROR: unable to download webpage: Temporary failure in name resolution",
		"ERROR: The read operation timed out",
	}

	for _, stderr := range casos {
		require.ErrorIs(t, classificar(nil, stderr), media.ErrNetwork, stderr)
	}
}

// A ordem da tabela importa: 429 e "login required" são causas mais
// específicas para respostas da mesma família e não podem ser engolidas pelo
// padrão de bloqueio.
func TestClassificarNaoDeixaBloqueioEngolirCausasMaisEspecificas(t *testing.T) {
	require.ErrorIs(t,
		classificar(nil, "ERROR: [Reddit] abc: HTTP Error 429: Too Many Requests"),
		media.ErrRateLimited)
	require.ErrorIs(t,
		classificar(nil, "ERROR: [vimeo] 1: The web client only works when logged-in. Use --cookies"),
		media.ErrAuthRequired)
}

// O detalhe técnico existe para o log e para o super admin; nunca é a mensagem
// que chega ao usuário comum.
func TestDetalheNaoVazaParaAMensagemDoUsuario(t *testing.T) {
	err := classificar(nil, "ERROR: [Reddit] abc: Unable to download webpage: HTTP Error 403: Forbidden")

	require.Equal(t, media.ErrBlocked.Error(), media.UserMessage(err))
	require.Contains(t, media.Detail(err), "403")
	require.NotContains(t, media.UserMessage(err), "403")
}

// A sessão precisa chegar ao processo na RESOLUÇÃO, e não só no download:
// Vimeo recusa a leitura de metadados sem login, e Reddit, X e Pinterest
// recusam o IP de datacenter.
func TestArgsMetadataPassaOArquivoDeSessao(t *testing.T) {
	provider := New(Config{Binary: "yt-dlp"})
	alvo, err := url.Parse("https://www.reddit.com/r/GTA6/s/abc")
	require.NoError(t, err)

	args := provider.argsMetadata(alvo, media.MetadataOptions{CookieFile: "/tmp/sessao/cookies.txt"})

	require.Contains(t, args, "--cookies")
	require.Contains(t, args, "/tmp/sessao/cookies.txt")

	// A URL fecha a lista, logo depois do "--".
	require.Equal(t, alvo.String(), args[len(args)-1])
	require.Equal(t, "--", args[len(args)-2])
}

// Sem conta cadastrada a resolução segue anônima: a maioria do conteúdo público
// não precisa de sessão, e exigir uma quebraria o que hoje funciona.
func TestArgsMetadataSemSessaoNaoPassaCookies(t *testing.T) {
	provider := New(Config{Binary: "yt-dlp"})
	alvo, err := url.Parse("https://youtu.be/abc")
	require.NoError(t, err)

	args := provider.argsMetadata(alvo, media.MetadataOptions{})

	require.NotContains(t, args, "--cookies")
}

// Impersonação só onde é preciso: o YouTube funciona sem ela, e ligá-la em toda
// plataforma só somaria uma dependência ao caminho crítico.
func TestImpersonacaoApenasNasPlataformasQueExigem(t *testing.T) {
	exigem := []media.Platform{
		media.PlatformReddit, media.PlatformTwitter, media.PlatformPinterest,
		media.PlatformVimeo, media.PlatformDailymotion,
	}
	for _, plataforma := range exigem {
		require.True(t, plataformasQueRecusamDatacenter[plataforma], plataforma)
	}

	naoExigem := []media.Platform{
		media.PlatformYouTube, media.PlatformTwitch, media.PlatformLinkedIn,
		media.PlatformSoundCloud, media.PlatformUnknown,
	}
	for _, plataforma := range naoExigem {
		require.False(t, plataformasQueRecusamDatacenter[plataforma], plataforma)
	}
}

// O User-Agent próprio e a impersonação são incompatíveis: imitando o Chrome, o
// yt-dlp já envia o cabeçalho correspondente, e sobrescrevê-lo deixaria o
// handshake e o User-Agent contando histórias diferentes — o que é, por si só,
// sinal de automação.
func TestArgsBaseNaoMisturaUserAgentComImpersonacao(t *testing.T) {
	provider := New(Config{Binary: binarioFalso(t, "yt-dlp", "2025.09.01"), UserAgent: "UA-do-projeto"})

	// Numa plataforma que não impersona, o User-Agent configurado vale.
	semImpersonacao := provider.argsBase(media.PlatformYouTube, faseExtracao)
	require.Contains(t, semImpersonacao, "--user-agent")
	require.Contains(t, semImpersonacao, "UA-do-projeto")

	// Onde impersona, os dois nunca aparecem juntos.
	comImpersonacao := provider.argsBase(media.PlatformReddit, faseExtracao)
	if contemArg(comImpersonacao, "--impersonate") {
		require.NotContains(t, comImpersonacao, "--user-agent",
			"User-Agent próprio e impersonação não podem coexistir")
	}
}

// `--impersonate` com alvo indisponível é ERRO FATAL no yt-dlp, não aviso. Uma
// imagem construída sem o extra curl-cffi não pode ter todo download das
// plataformas da lista quebrado por causa disso.
func TestSemSuporteNaoPassaImpersonacao(t *testing.T) {
	// Um binário que não entende --list-impersonate-targets: a sondagem falha
	// e a impersonação fica desligada.
	provider := New(Config{Binary: filepath.Join(t.TempDir(), "binario-que-nao-existe")})

	require.Empty(t, provider.argsImpersonacao(media.PlatformReddit))
	require.False(t, provider.usaImpersonacao(media.PlatformReddit))
}

func contemArg(args []string, alvo string) bool {
	for _, arg := range args {
		if arg == alvo {
			return true
		}
	}
	return false
}

// A sondagem é por provider: um binário sem suporte não pode contaminar outro
// que tenha, nem o resultado depender de qual teste rodou primeiro.
func TestSondagemDeImpersonacaoEhPorProvider(t *testing.T) {
	semSuporte := New(Config{Binary: filepath.Join(t.TempDir(), "inexistente")})
	require.False(t, semSuporte.suportaImpersonacao())

	// Um binário que responde à sondagem listando alvos de Chrome.
	comSuporte := New(Config{
		Binary: binarioFalso(t, "yt-dlp", "Chrome-136      Macos-15     curl_cffi"),
	})
	require.True(t, comSuporte.suportaImpersonacao())
	require.Contains(t, comSuporte.argsImpersonacao(media.PlatformReddit), "--impersonate")

	// E o primeiro continua sem suporte depois disso.
	require.False(t, semSuporte.suportaImpersonacao())
	require.Empty(t, semSuporte.argsImpersonacao(media.PlatformReddit))
}

// Mesmo com suporte, plataforma fora da lista não recebe o argumento.
func TestImpersonacaoNaoVazaParaPlataformaForaDaLista(t *testing.T) {
	provider := New(Config{
		Binary: binarioFalso(t, "yt-dlp", "Chrome-136      Macos-15     curl_cffi"),
	})

	require.Empty(t, provider.argsImpersonacao(media.PlatformYouTube))
	require.NotContains(t, provider.argsBase(media.PlatformYouTube, faseExtracao), "--impersonate")
	require.Contains(t, provider.argsBase(media.PlatformReddit, faseExtracao), "--impersonate")
}

// O caso que mais confunde: a API resolve o link e o worker falha ao baixar,
// porque só uma das duas imagens foi reconstruída com curl-cffi. Sem esta
// explicação, os dois casos mostram "403" e nada distingue "a plataforma
// bloqueou nosso IP" de "esta imagem está sem a dependência".
func TestExplicarBloqueioApontaImpersonacaoAusente(t *testing.T) {
	semSuporte := New(Config{Binary: filepath.Join(t.TempDir(), "inexistente")})
	bloqueio := &media.Error{Kind: media.ErrBlocked, Detail: "HTTP Error 403: Blocked"}

	explicado := semSuporte.explicarBloqueio(bloqueio, media.PlatformTwitter)

	require.ErrorIs(t, explicado, media.ErrBlocked, "o tipo do erro não muda")
	require.Contains(t, media.Detail(explicado), "403", "o detalhe original é preservado")
	require.Contains(t, media.Detail(explicado), "curl-cffi")
	// E a mensagem pública continua genérica, como manda a regra.
	require.Equal(t, media.PublicMessage(media.ErrBlocked), media.PublicMessage(explicado))
}

// Com impersonação disponível, o bloqueio é da plataforma mesmo: culpar a
// dependência mandaria o administrador procurar no lugar errado.
func TestExplicarBloqueioNaoCulpaDependenciaQuandoElaExiste(t *testing.T) {
	comSuporte := New(Config{
		Binary: binarioFalso(t, "yt-dlp", "Chrome-136      Macos-15     curl_cffi"),
	})
	bloqueio := &media.Error{Kind: media.ErrBlocked, Detail: "HTTP Error 403: Blocked"}

	explicado := comSuporte.explicarBloqueio(bloqueio, media.PlatformTwitter)

	require.NotContains(t, media.Detail(explicado), "curl-cffi")
}

// Plataforma que não usa impersonação, e erro que não é bloqueio, passam
// intactos.
func TestExplicarBloqueioNaoTocaNoQueNaoEhSeuCaso(t *testing.T) {
	semSuporte := New(Config{Binary: filepath.Join(t.TempDir(), "inexistente")})

	bloqueioNoYoutube := &media.Error{Kind: media.ErrBlocked, Detail: "403"}
	require.NotContains(t,
		media.Detail(semSuporte.explicarBloqueio(bloqueioNoYoutube, media.PlatformYouTube)),
		"curl-cffi")

	privado := &media.Error{Kind: media.ErrContentPrivate, Detail: "private video"}
	require.NotContains(t,
		media.Detail(semSuporte.explicarBloqueio(privado, media.PlatformTwitter)),
		"curl-cffi")

	require.Nil(t, semSuporte.explicarBloqueio(nil, media.PlatformTwitter))
}

// A separação que torna um proxy residencial viável: a extração custa dezenas
// de KB e pode sair por ele; a mídia custa dezenas de MB e nunca pode.
func TestProxyDeResolucaoNuncaCarregaAMidia(t *testing.T) {
	provider := New(Config{
		Binary:          binarioFalso(t, "yt-dlp", "Chrome-136  curl_cffi"),
		ProxyURL:        "",
		ResolveProxyURL: "http://proxy-residencial:8080",
	})

	extracao := provider.proxyDa(media.PlatformTwitter, faseExtracao)
	require.Equal(t, "http://proxy-residencial:8080", extracao,
		"a extração de uma plataforma bloqueada deve sair pelo proxy")

	midia := provider.proxyDa(media.PlatformTwitter, faseMidia)
	require.Empty(t, midia,
		"os bytes do vídeo NUNCA podem sair pelo proxy de resolução: queimariam a cota")
}

// Plataforma que não é bloqueada não gasta cota nenhuma.
func TestProxyDeResolucaoNaoEhUsadoOndeNaoPrecisa(t *testing.T) {
	provider := New(Config{
		Binary:          binarioFalso(t, "yt-dlp", "x"),
		ResolveProxyURL: "http://proxy-residencial:8080",
	})

	require.Empty(t, provider.proxyDa(media.PlatformYouTube, faseExtracao))
	require.Empty(t, provider.proxyDa(media.PlatformYouTube, faseMidia))
}

// O proxy geral continua valendo para tudo, quando configurado: ele é outra
// decisão, com outra finalidade.
func TestProxyGeralContinuaValendoNasDuasFases(t *testing.T) {
	provider := New(Config{
		Binary:   binarioFalso(t, "yt-dlp", "x"),
		ProxyURL: "http://proxy-geral:3128",
	})

	require.Equal(t, "http://proxy-geral:3128", provider.proxyDa(media.PlatformTwitter, faseExtracao))
	require.Equal(t, "http://proxy-geral:3128", provider.proxyDa(media.PlatformTwitter, faseMidia))
	require.Equal(t, "http://proxy-geral:3128", provider.proxyDa(media.PlatformYouTube, faseMidia))
}

// Com a extração pronta, o download não pode voltar à plataforma: passar a URL
// junto faria exatamente isso, e o proxy teria sido em vão.
func TestArgsArquivoComExtracaoProntaNaoPassaAUrl(t *testing.T) {
	provider := New(Config{Binary: "yt-dlp"})
	req := media.Request{
		URL: "https://x.com/user/status/1", Filename: "media_abc",
		Kind: media.KindVideo, FormatID: "hls-720", MaxHeight: 720,
	}

	args := provider.argsArquivo(req, t.TempDir(), "/tmp/dl/extracao.info.json")

	require.Contains(t, args, "--load-info-json")
	require.Contains(t, args, "/tmp/dl/extracao.info.json")
	require.NotContains(t, args, req.URL, "a URL faria o yt-dlp extrair de novo")
	// A escolha de formato continua valendo sobre a extração carregada.
	require.Contains(t, args, "-f")
}

// Sem extração separada, nada muda: é o caminho de sempre.
func TestArgsArquivoSemExtracaoUsaAUrl(t *testing.T) {
	provider := New(Config{Binary: "yt-dlp"})
	req := media.Request{URL: "https://youtu.be/abc", Filename: "media_abc", Kind: media.KindVideo}

	args := provider.argsArquivo(req, t.TempDir(), "")

	require.NotContains(t, args, "--load-info-json")
	require.Equal(t, req.URL, args[len(args)-1])
	require.Equal(t, "--", args[len(args)-2])
}

// O arquivo de extração fica no mesmo diretório do download; ele não pode ser
// confundido com o resultado.
func TestExtracaoNaoEhConfundidaComOArquivoFinal(t *testing.T) {
	dir := t.TempDir()
	base := "media_abc"

	require.NoError(t, os.WriteFile(filepath.Join(dir, nomeInfoJSON), []byte(`{"id":"x"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".mp4"), []byte("conteudo do video"), 0o600))

	caminho, _, err := arquivoProduzido(dir, base)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, base+".mp4"), caminho)
}
