package ytdlp

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// padroesErro traduz a saída do yt-dlp para os erros de domínio. A ordem
// importa: o primeiro padrão que casar vence, então os mais específicos vêm
// antes dos genéricos.
//
// O texto do yt-dlp muda entre versões, e é por isso que ele fica confinado
// aqui: uma frase nova exige mudar esta tabela, e nada mais.
var padroesErro = []struct {
	trechos []string
	tipo    error
}{
	// Autenticação vem antes de "unavailable" porque a mensagem do YouTube
	// costuma trazer as duas ideias na mesma linha.
	{[]string{"sign in to confirm your age", "age-restricted", "age restricted"}, media.ErrAuthRequired},
	{[]string{"sign in to confirm you're not a bot", "confirm you're not a bot"}, media.ErrAuthRequired},
	{[]string{"login required", "please sign in", "authentication", "requires authentication",
		"cookies are no longer valid", "use --cookies"}, media.ErrAuthRequired},

	{[]string{"private video", "this video is private", "private account",
		"is private", "unable to access private"}, media.ErrContentPrivate},

	{[]string{"members-only", "join this channel", "paid members"}, media.ErrAuthRequired},

	// "available in your country" cobre as duas formas que o yt-dlp emite:
	// "is not available in your country" e "has not made this video available
	// in your country".
	{[]string{"available in your country", "geo restricted", "geo-restricted",
		"blocked it in your country", "not available from your location",
		"available in your location"}, media.ErrGeoBlocked},

	{[]string{"live event will begin", "is live", "live stream", "livestream",
		"currently live"}, media.ErrLiveContent},

	// "does not exist" sem o prefixo "video": o yt-dlp coloca o ID no meio da
	// frase ("Video 2000000000 does not exist"), e exigir as duas palavras
	// juntas deixava o caso cair no erro genérico.
	{[]string{"video unavailable", "this video has been removed", "no longer available",
		"content isn't available", "does not exist", "not found",
		"no video could be found", "no media found",
		"has been terminated", "account has been suspended", "404"}, media.ErrContentUnavailable},

	{[]string{"requested format is not available", "requested format not available",
		"no video formats found"}, media.ErrFormatUnavailable},

	{[]string{"http error 429", "too many requests", "rate limit", "rate-limit"}, media.ErrRateLimited},

	{[]string{"unsupported url", "no suitable extractor", "is not a valid url"}, media.ErrUnsupportedPlatform},

	{[]string{"no space left on device", "disk quota exceeded"}, media.ErrDownloadFailed},
}

// classificar traduz o erro de execução e o stderr em um erro de domínio. O
// stderr entra como Detail: fica no log e no painel, nunca na tela do usuário.
func classificar(err error, stderr string) error {
	switch {
	case errors.Is(err, context.Canceled):
		return &media.Error{Kind: media.ErrCanceled}
	case errors.Is(err, context.DeadlineExceeded):
		return &media.Error{Kind: media.ErrTimeout}
	}

	minusculo := strings.ToLower(stderr)
	for _, padrao := range padroesErro {
		for _, trecho := range padrao.trechos {
			if strings.Contains(minusculo, trecho) {
				return &media.Error{Kind: padrao.tipo, Detail: resumirStderr(stderr)}
			}
		}
	}

	// Binário ausente ou sem permissão: é falha de infraestrutura, não do
	// conteúdo, e o administrador precisa saber a diferença.
	var erroExec *exec.Error
	if errors.As(err, &erroExec) {
		return &media.Error{Kind: media.ErrProviderUnavailable, Detail: erroExec.Error()}
	}

	detalhe := resumirStderr(stderr)
	if detalhe == "" && err != nil {
		detalhe = err.Error()
	}
	return &media.Error{Kind: media.ErrDownloadFailed, Detail: detalhe}
}

// resumirStderr fica com as linhas que interessam. O yt-dlp mistura avisos
// irrelevantes com o erro real; guardar tudo enche o log e a coluna de erro.
func resumirStderr(stderr string) string {
	const maxLinhas = 4
	const maxTamanho = 500

	var uteis []string
	for _, linha := range strings.Split(stderr, "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		minuscula := strings.ToLower(linha)
		// Avisos sobre runtime JS e formatos ausentes aparecem em execuções que
		// terminam bem; não são a causa quando algo falha.
		if strings.HasPrefix(minuscula, "warning:") {
			continue
		}
		uteis = append(uteis, linha)
		if len(uteis) >= maxLinhas {
			break
		}
	}

	resumo := strings.Join(uteis, " | ")
	if len(resumo) > maxTamanho {
		resumo = resumo[:maxTamanho] + "…"
	}
	return resumo
}
