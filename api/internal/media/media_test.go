package media

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeURLAceitaVariantes(t *testing.T) {
	casos := []struct {
		entrada    string
		plataforma Platform
	}{
		{"https://www.youtube.com/watch?v=abc", PlatformYouTube},
		{"https://youtu.be/abc", PlatformYouTube},
		{"https://m.youtube.com/watch?v=abc", PlatformYouTube},
		{"https://music.youtube.com/watch?v=abc", PlatformYouTube},
		// Sem esquema: é como o usuário cola da barra de endereços.
		{"youtu.be/abc", PlatformYouTube},
		{"  https://youtu.be/abc  ", PlatformYouTube},
		{"https://vm.tiktok.com/ZM123/", PlatformTikTok},
		{"https://www.tiktok.com/@user/video/123", PlatformTikTok},
		{"https://www.instagram.com/reel/abc/", PlatformInstagram},
		{"https://fb.watch/abc/", PlatformFacebook},
		{"https://x.com/user/status/123", PlatformTwitter},
		{"https://twitter.com/user/status/123", PlatformTwitter},
		{"https://old.reddit.com/r/videos/comments/abc/", PlatformReddit},
		{"https://www.twitch.tv/videos/123", PlatformTwitch},
		{"https://vimeo.com/123", PlatformVimeo},
		{"https://example.com/video.mp4", PlatformUnknown},
	}

	for _, caso := range casos {
		normalizada, parsed, err := NormalizeURL(caso.entrada)
		require.NoError(t, err, caso.entrada)
		require.NotEmpty(t, normalizada)
		require.Equal(t, caso.plataforma, PlatformFor(parsed), caso.entrada)
	}
}

func TestNormalizeURLRecusaEntradaPerigosa(t *testing.T) {
	perigosas := []string{
		"",
		"   ",
		// Leitura de arquivo local pelo processo de download.
		"file:///etc/passwd",
		"ftp://exemplo.com/a.mp4",
		"javascript:alert(1)",
		// Rede interna: o servidor viraria um proxy para dentro da stack.
		"http://localhost/admin",
		"http://127.0.0.1:8080/",
		"http://10.0.0.5/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://redis:6379/",
		"http://postgres/",
		"http://browser:9223/internal/sessions",
		"http://metadata.google.internal/",
		"https://algo.internal/video",
		// Caractere de controle: quebra de log e injeção em argumento.
		"https://exemplo.com/\nvideo",
		"https://exemplo.com/\x00",
	}

	for _, entrada := range perigosas {
		_, _, err := NormalizeURL(entrada)
		require.Error(t, err, "deveria recusar: %q", entrada)
		require.Equal(t, ErrInvalidURL.Error(), UserMessage(err), entrada)
	}
}

func TestNormalizeURLNaoCasaDominioFalso(t *testing.T) {
	// O ataque clássico contra strings.Contains: o domínio real é outro.
	falsos := []string{
		"https://youtube.com.site-falso.net/video",
		"https://tiktok.com.evil.example/v",
		"https://naoyoutube.com/watch?v=1",
	}

	for _, entrada := range falsos {
		_, parsed, err := NormalizeURL(entrada)
		require.NoError(t, err)
		require.Equal(t, PlatformUnknown, PlatformFor(parsed), entrada)
	}
}

func TestNormalizeURLDescartaCredenciaisEFragmento(t *testing.T) {
	// Credencial embutida iria parar no banco, no log e na tela.
	normalizada, _, err := NormalizeURL("https://user:senha@vimeo.com/123#trecho")
	require.NoError(t, err)
	require.NotContains(t, normalizada, "senha")
	require.NotContains(t, normalizada, "user:")
	require.NotContains(t, normalizada, "#trecho")
}

func TestNormalizeURLRecusaTamanhoAbsurdo(t *testing.T) {
	longa := "https://exemplo.com/" + string(make([]byte, maxURLLength))
	_, _, err := NormalizeURL(longa)
	require.ErrorIs(t, err, ErrInvalidURL)
}

// providerFalso exercita o registry sem tocar em processo nenhum.
type providerFalso struct {
	nome     string
	aceita   bool
	metadata *Metadata
	err      error
	chamadas int
}

func (p *providerFalso) Name() string            { return p.nome }
func (p *providerFalso) CanHandle(*url.URL) bool { return p.aceita }

func (p *providerFalso) Metadata(context.Context, *url.URL, MetadataOptions) (*Metadata, error) {
	p.chamadas++
	if p.err != nil {
		return nil, p.err
	}
	copia := *p.metadata
	return &copia, nil
}

func (p *providerFalso) Download(context.Context, Request, ProgressFunc) (*Result, error) {
	return nil, p.err
}

func (p *providerFalso) Health(context.Context) Health {
	return Health{Provider: p.nome, Available: p.err == nil}
}

func TestRegistryUsaOPrimeiroQueAceita(t *testing.T) {
	primeiro := &providerFalso{nome: "a", aceita: true, metadata: &Metadata{Title: "de A"}}
	segundo := &providerFalso{nome: "b", aceita: true, metadata: &Metadata{Title: "de B"}}

	registry := NewRegistry(primeiro, segundo)
	metadata, _, err := registry.Metadata(context.Background(), "https://youtu.be/abc", MetadataOptions{})

	require.NoError(t, err)
	require.Equal(t, "de A", metadata.Title)
	require.Equal(t, "a", metadata.Provider)
	require.Equal(t, PlatformYouTube, metadata.Platform)
	require.Zero(t, segundo.chamadas, "o segundo provider não devia ter sido chamado")
}

func TestRegistryCaiParaOProximoQuandoOPrimeiroFalha(t *testing.T) {
	// Falha de disponibilidade: o próximo provider deve assumir.
	primeiro := &providerFalso{nome: "a", aceita: true, err: &Error{Kind: ErrProviderUnavailable}}
	segundo := &providerFalso{nome: "b", aceita: true, metadata: &Metadata{Title: "de B"}}

	registry := NewRegistry(primeiro, segundo)
	metadata, _, err := registry.Metadata(context.Background(), "https://vimeo.com/1", MetadataOptions{})

	require.NoError(t, err)
	require.Equal(t, "b", metadata.Provider)
	require.Equal(t, 1, segundo.chamadas)
}

func TestRegistryNaoInsisteQuandoOProblemaEODoConteudo(t *testing.T) {
	// Vídeo privado é privado para todo provider: insistir só gastaria tempo e
	// requisições contra a plataforma.
	for _, tipo := range []error{ErrContentPrivate, ErrContentUnavailable, ErrLiveContent, ErrAuthRequired} {
		primeiro := &providerFalso{nome: "a", aceita: true, err: &Error{Kind: tipo}}
		segundo := &providerFalso{nome: "b", aceita: true, metadata: &Metadata{Title: "de B"}}

		registry := NewRegistry(primeiro, segundo)
		_, _, err := registry.Metadata(context.Background(), "https://vimeo.com/1", MetadataOptions{})

		require.ErrorIs(t, err, tipo)
		require.Zero(t, segundo.chamadas, "não devia tentar outro provider para %v", tipo)
	}
}

func TestRegistryRecusaQuandoNinguemAceita(t *testing.T) {
	registry := NewRegistry(&providerFalso{nome: "a", aceita: false})
	_, _, err := registry.Metadata(context.Background(), "https://exemplo.com/v", MetadataOptions{})

	require.ErrorIs(t, err, ErrUnsupportedPlatform)
}

func TestUserMessageNaoVazaDetalheTecnico(t *testing.T) {
	err := &Error{
		Kind:   ErrContentUnavailable,
		Detail: "ERROR: [youtube] abc: /tmp/download/xyz.part não encontrado",
	}

	mensagem := UserMessage(err)
	require.Equal(t, ErrContentUnavailable.Error(), mensagem)
	require.NotContains(t, mensagem, "/tmp")
	require.NotContains(t, mensagem, "youtube]")

	// Erro de fora do domínio não pode vazar o texto original.
	require.Equal(t, ErrDownloadFailed.Error(), UserMessage(errors.New("exec: \"yt-dlp\": not found")))
}

// Kwai é reconhecido pela URL, mas o yt-dlp não tem extractor para ele. Deixar
// o caminho genérico tentar custava dezenas de segundos para terminar em
// "Unsupported URL" — recusar aqui responde na hora e dizendo qual plataforma é.
func TestMetadataRecusaPlataformaSemExtractor(t *testing.T) {
	registry := NewRegistry(&providerFalso{nome: "a", aceita: true, metadata: &Metadata{Title: "nunca"}})

	_, _, err := registry.Metadata(context.Background(),
		"https://www.kwai.com/@canal/video/123", MetadataOptions{})

	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("esperado ErrUnsupportedPlatform, obtido %v", err)
	}
	if !strings.Contains(UserMessage(err), "suportada") {
		t.Errorf("a mensagem ao usuário deveria explicar a falta de suporte: %q", UserMessage(err))
	}
}

func TestResolveRecusaPlataformaSemExtractor(t *testing.T) {
	registry := NewRegistry(&providerFalso{nome: "a", aceita: true})

	plataforma, provider, _, err := registry.Resolve("https://www.kwai.com/@canal/video/123")

	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("esperado ErrUnsupportedPlatform, obtido %v", err)
	}
	// A plataforma continua identificada: é o que permite dizer QUAL não é
	// suportada em vez de um genérico "esta URL não funciona".
	if plataforma != PlatformKwai {
		t.Errorf("plataforma esperada %q, obtida %q", PlatformKwai, plataforma)
	}
	if provider != nil {
		t.Error("nenhum provider deveria ser escolhido")
	}
}

// As plataformas que têm extractor seguem passando: a recusa acima não pode
// virar uma peneira que derruba o que funciona.
func TestPlataformasComSuporteContinuamPassando(t *testing.T) {
	comSuporte := []Platform{
		PlatformYouTube, PlatformVimeo, PlatformReddit, PlatformTwitter,
		PlatformPinterest, PlatformDailymotion, PlatformLinkedIn, PlatformInstagram,
	}
	for _, plataforma := range comSuporte {
		if !plataforma.TemSuporte() {
			t.Errorf("%s deveria ter suporte", plataforma)
		}
	}
	if PlatformKwai.TemSuporte() {
		t.Error("Kwai não tem extractor no yt-dlp")
	}
}
