package browser

import "sort"

// PerfilPlataforma reúne tudo que muda entre gerenciar uma conta do YouTube e
// uma do Vimeo: onde o navegador remoto abre o login, quais domínios de cookie
// podem sair do container e quais nomes de cookie significam "esta sessão está
// autenticada".
//
// Está no pacote `browser` porque os dois lados precisam concordar: o serviço
// de navegador usa isto para exportar a sessão, e o downloader usa para decidir
// se a sessão vale alguma coisa antes de gastar uma requisição.
type PerfilPlataforma struct {
	// ID casa com media.Platform, para o download saber que conta pedir.
	ID    string
	Label string

	// LoginURL é a página que o navegador remoto abre. Sempre a tela de login
	// da própria plataforma: o administrador digita as credenciais lá, e nada
	// disso passa por este sistema.
	LoginURL string

	// CheckURL é uma página que só responde para quem está logado. Vazio
	// significa que o health check se limita à presença dos cookies — ver
	// VerificaPorHTTP.
	CheckURL string

	// MarcaAutenticada é um trecho que só aparece no HTML de quem está logado.
	// Só faz sentido junto de CheckURL.
	MarcaAutenticada string

	// HostDeLogin identifica o redirecionamento para a tela de login, que é
	// como a maioria das plataformas responde a uma sessão expirada.
	HostDeLogin string

	// CookieDomains restringe o que é exportado do perfil. Qualquer outro site
	// que o administrador tenha aberto no navegador remoto fica de fora.
	CookieDomains []string

	// CookiesDeSessao são os nomes que a plataforma emite para uma sessão
	// logada. Um perfil sem nenhum deles está deslogado, e isso é detectável
	// sem tocar na rede.
	CookiesDeSessao []string

	// MetadadosExigemSessao diz se até a LEITURA de metadados precisa de conta.
	//
	// Buscar os cookies custa caro — pode subir um Chrome headless sobre o
	// perfil —, e a resolução roda dentro da requisição HTTP do usuário. O
	// custo é pago onde resolver anônimo comprovadamente falha, e amortizado
	// pelo cache de jar do pacote ytaccounts: a primeira resolução da conta
	// paga o Chrome, as seguintes reaproveitam o mesmo jar.
	MetadadosExigemSessao bool
}

// VerificaPorHTTP diz se o health check pode confirmar a sessão pela rede.
//
// Só o YouTube tem uma marca confiável em HTML servido a um cliente HTTP
// comum. As outras plataformas respondem a um GET sem fingerprint de navegador
// com muro de bot ou uma casca vazia de JavaScript — e interpretar isso como
// "deslogado" derrubaria contas perfeitamente boas do rodízio. Para elas o
// veredito vem de duas fontes melhores: a presença dos cookies de sessão e o
// que o download de verdade reporta de volta.
func (p PerfilPlataforma) VerificaPorHTTP() bool {
	return p.CheckURL != "" && p.MarcaAutenticada != ""
}

// PlataformaPadrao é usada quando nada foi informado. O YouTube é o padrão
// porque foi a única plataforma até esta versão: contas gravadas antes dela não
// têm plataforma registrada e são todas dele.
const PlataformaPadrao = "youtube"

var perfis = map[string]PerfilPlataforma{
	"youtube": {
		ID:               "youtube",
		Label:            "YouTube",
		LoginURL:         "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Fwww.youtube.com%2F",
		CheckURL:         "https://www.youtube.com/account",
		MarcaAutenticada: `"LOGGED_IN"\s*:\s*true`,
		HostDeLogin:      "accounts.google.com",
		CookieDomains:    []string{"youtube.com", "google.com", "googlevideo.com", "ytimg.com"},
		CookiesDeSessao: []string{
			"SID", "__Secure-1PSID", "__Secure-3PSID",
			"LOGIN_INFO", "SAPISID", "__Secure-1PAPISID",
		},
		// O YouTube resolvia anônimo quando este perfil foi escrito, e por
		// isso a resolução não gastava uma sessão com ele. Não resolve mais de
		// um IP de datacenter: a primeira requisição volta com "Sign in to
		// confirm you're not a bot", ANTES de o usuário chegar à escolha de
		// qualidade. O download já emprestava a conta e passava; só a leitura
		// de metadados ia sem ela, e era ela que barrava todo mundo na tela.
		MetadadosExigemSessao: true,
	},
	"vimeo": {
		ID:          "vimeo",
		Label:       "Vimeo",
		LoginURL:    "https://vimeo.com/log_in",
		HostDeLogin: "vimeo.com/log_in",
		// O Vimeo recusa até a resolução de metadados sem sessão: "the web
		// client only works when logged-in". É a plataforma que mais depende
		// desta funcionalidade.
		CookieDomains:         []string{"vimeo.com", "vimeocdn.com"},
		CookiesDeSessao:       []string{"vimeo", "is_logged_in", "vimeo_gdpr_optin"},
		MetadadosExigemSessao: true,
	},
	"reddit": {
		ID:              "reddit",
		Label:           "Reddit",
		LoginURL:        "https://www.reddit.com/login/",
		HostDeLogin:     "reddit.com/login",
		CookieDomains:   []string{"reddit.com", "redd.it", "redditstatic.com", "redditmedia.com"},
		CookiesDeSessao: []string{"reddit_session", "token_v2"},
		// Atende normalmente de uma conexão residencial e bloqueia o IP do
		// datacenter já na leitura dos metadados. A sessão é o que devolve o
		// acesso.
		MetadadosExigemSessao: true,
	},
	"twitter": {
		ID:                    "twitter",
		Label:                 "X (Twitter)",
		LoginURL:              "https://x.com/i/flow/login",
		HostDeLogin:           "x.com/i/flow/login",
		CookieDomains:         []string{"x.com", "twitter.com", "twimg.com"},
		CookiesDeSessao:       []string{"auth_token", "ct0"},
		MetadadosExigemSessao: true,
	},
	"pinterest": {
		ID:                    "pinterest",
		Label:                 "Pinterest",
		LoginURL:              "https://www.pinterest.com/login/",
		HostDeLogin:           "pinterest.com/login",
		CookieDomains:         []string{"pinterest.com", "pinimg.com"},
		CookiesDeSessao:       []string{"_pinterest_sess", "_auth"},
		MetadadosExigemSessao: true,
	},
	"instagram": {
		ID:          "instagram",
		Label:       "Instagram",
		LoginURL:    "https://www.instagram.com/accounts/login/",
		HostDeLogin: "instagram.com/accounts/login",
		// O login do Instagram passa pelo Facebook; sem o domínio dele a
		// sessão exportada fica pela metade.
		CookieDomains:         []string{"instagram.com", "cdninstagram.com", "facebook.com"},
		CookiesDeSessao:       []string{"sessionid", "ds_user_id"},
		MetadadosExigemSessao: true,
	},
	"facebook": {
		ID:                    "facebook",
		Label:                 "Facebook",
		LoginURL:              "https://www.facebook.com/login/",
		HostDeLogin:           "facebook.com/login",
		CookieDomains:         []string{"facebook.com", "fbcdn.net"},
		CookiesDeSessao:       []string{"c_user", "xs"},
		MetadadosExigemSessao: true,
	},
	"tiktok": {
		ID:                    "tiktok",
		Label:                 "TikTok",
		LoginURL:              "https://www.tiktok.com/login",
		HostDeLogin:           "tiktok.com/login",
		CookieDomains:         []string{"tiktok.com", "tiktokcdn.com", "byteoversea.com"},
		CookiesDeSessao:       []string{"sessionid", "sid_tt", "sid_guard"},
		MetadadosExigemSessao: true,
	},
}

// PerfilDe devolve o perfil da plataforma. Uma plataforma desconhecida não é
// erro do chamador: contas gravadas antes desta versão não têm plataforma, e
// caem no padrão.
func PerfilDe(plataforma string) PerfilPlataforma {
	if perfil, ok := perfis[plataforma]; ok {
		return perfil
	}
	return perfis[PlataformaPadrao]
}

// PlataformaSuportada diz se há um perfil para esta plataforma. É o que impede
// o painel de aceitar uma conta que ninguém saberia autenticar.
func PlataformaSuportada(plataforma string) bool {
	_, ok := perfis[plataforma]
	return ok
}

// PlataformasSuportadas lista os perfis para o painel montar a escolha.
func PlataformasSuportadas() []PerfilPlataforma {
	lista := make([]PerfilPlataforma, 0, len(perfis))
	for _, perfil := range perfis {
		lista = append(lista, perfil)
	}
	// O YouTube primeiro por ser o mais usado; o resto em ordem alfabética,
	// para a tela não embaralhar a cada requisição.
	sort.Slice(lista, func(i, j int) bool {
		if (lista[i].ID == PlataformaPadrao) != (lista[j].ID == PlataformaPadrao) {
			return lista[i].ID == PlataformaPadrao
		}
		return lista[i].Label < lista[j].Label
	})
	return lista
}
