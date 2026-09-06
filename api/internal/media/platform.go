package media

import (
	"net/url"
	"strings"
)

// Platform é a origem do conteúdo. É um conceito separado do provider: hoje
// todas as plataformas são atendidas pelo yt-dlp, e amanhã uma delas pode ter
// um provider dedicado sem que nada mais mude.
type Platform string

const (
	PlatformYouTube     Platform = "youtube"
	PlatformTikTok      Platform = "tiktok"
	PlatformInstagram   Platform = "instagram"
	PlatformFacebook    Platform = "facebook"
	PlatformTwitter     Platform = "twitter"
	PlatformReddit      Platform = "reddit"
	PlatformTwitch      Platform = "twitch"
	PlatformVimeo       Platform = "vimeo"
	PlatformDailymotion Platform = "dailymotion"
	PlatformSoundCloud  Platform = "soundcloud"
	PlatformKwai        Platform = "kwai"
	PlatformPinterest   Platform = "pinterest"
	PlatformLinkedIn    Platform = "linkedin"
	// PlatformUnknown cobre o resto do que o yt-dlp entende. O download segue
	// normalmente; só não há rótulo bonito para mostrar na tela.
	PlatformUnknown Platform = "unknown"
)

// Label é o nome que aparece na interface.
func (p Platform) Label() string {
	switch p {
	case PlatformYouTube:
		return "YouTube"
	case PlatformTikTok:
		return "TikTok"
	case PlatformInstagram:
		return "Instagram"
	case PlatformFacebook:
		return "Facebook"
	case PlatformTwitter:
		return "X (Twitter)"
	case PlatformReddit:
		return "Reddit"
	case PlatformTwitch:
		return "Twitch"
	case PlatformVimeo:
		return "Vimeo"
	case PlatformDailymotion:
		return "Dailymotion"
	case PlatformSoundCloud:
		return "SoundCloud"
	case PlatformKwai:
		return "Kwai"
	case PlatformPinterest:
		return "Pinterest"
	case PlatformLinkedIn:
		return "LinkedIn"
	default:
		return "Outra plataforma"
	}
}

// dominios mapeia o domínio REGISTRÁVEL para a plataforma. A chave nunca inclui
// subdomínio: `m.youtube.com`, `www.youtube.com` e `music.youtube.com` casam
// todos com `youtube.com` pela regra de sufixo aplicada em PlatformFor.
//
// Um mapa de domínios, e não strings.Contains: `Contains(url, "youtube.com")`
// aceitaria `youtube.com.site-falso.net`, que não é o YouTube.
var dominios = map[string]Platform{
	"youtube.com":          PlatformYouTube,
	"youtu.be":             PlatformYouTube,
	"youtube-nocookie.com": PlatformYouTube,
	"tiktok.com":           PlatformTikTok,
	"instagram.com":        PlatformInstagram,
	"instagr.am":           PlatformInstagram,
	"facebook.com":         PlatformFacebook,
	"fb.watch":             PlatformFacebook,
	"fb.com":               PlatformFacebook,
	"twitter.com":          PlatformTwitter,
	"x.com":                PlatformTwitter,
	"t.co":                 PlatformTwitter,
	"reddit.com":           PlatformReddit,
	"redd.it":              PlatformReddit,
	"twitch.tv":            PlatformTwitch,
	"vimeo.com":            PlatformVimeo,
	"dailymotion.com":      PlatformDailymotion,
	"dai.ly":               PlatformDailymotion,
	"soundcloud.com":       PlatformSoundCloud,
	"kwai.com":             PlatformKwai,
	"kwai-video.com":       PlatformKwai,
	"pinterest.com":        PlatformPinterest,
	"pin.it":               PlatformPinterest,
	"linkedin.com":         PlatformLinkedIn,
}

// PlatformFor identifica a plataforma de uma URL já normalizada.
func PlatformFor(parsed *url.URL) Platform {
	if parsed == nil {
		return PlatformUnknown
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")

	if plataforma, ok := dominios[host]; ok {
		return plataforma
	}

	// Subdomínios: `m.youtube.com`, `vm.tiktok.com`, `old.reddit.com`. O ponto
	// antes do domínio é o que impede `youtube.com.site-falso.net` de casar.
	for dominio, plataforma := range dominios {
		if strings.HasSuffix(host, "."+dominio) {
			return plataforma
		}
	}

	return PlatformUnknown
}

// semExtractor lista as plataformas que o mecanismo reconhece pela URL mas não
// sabe baixar: não existe extractor para elas, e o caminho genérico só descobre
// isso depois de buscar a página e falhar.
//
// Recusar cedo troca uma espera de dezenas de segundos terminando em erro
// obscuro por uma resposta imediata e honesta. A lista fica aqui, junto do
// resto do que se sabe sobre a plataforma.
var semExtractor = map[Platform]bool{
	PlatformKwai: true,
}

// TemSuporte informa se vale a pena sequer tentar. Uma plataforma sem suporte
// continua sendo reconhecida — é isso que permite dizer QUAL plataforma não é
// suportada, em vez de "esta URL não funciona".
func (p Platform) TemSuporte() bool {
	return !semExtractor[p]
}

// KnownPlatforms lista as plataformas com rótulo próprio, para o painel
// administrativo. Não é a lista do que o yt-dlp consegue baixar — essa é bem
// maior e cresce a cada versão dele.
func KnownPlatforms() []Platform {
	return []Platform{
		PlatformYouTube, PlatformTikTok, PlatformInstagram, PlatformFacebook,
		PlatformTwitter, PlatformReddit, PlatformTwitch, PlatformVimeo,
		PlatformDailymotion, PlatformSoundCloud, PlatformKwai, PlatformPinterest,
		PlatformLinkedIn,
	}
}
