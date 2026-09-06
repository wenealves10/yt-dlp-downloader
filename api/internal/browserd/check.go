package browserd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
)

const (
	checkUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	checkBodyLimit = 1 << 20
)

var emailPattern = regexp.MustCompile(`"email"\s*:\s*"([^"@]+@[^"]+)"`)

// marcasAutenticadas guarda as expressões compiladas por plataforma. Compilar a
// cada checagem seria desperdício, e um padrão inválido precisa quebrar na
// subida do processo, não no meio de um health check.
var marcasAutenticadas = compilarMarcas()

func compilarMarcas() map[string]*regexp.Regexp {
	marcas := make(map[string]*regexp.Regexp)
	for _, perfil := range browser.PlataformasSuportadas() {
		if perfil.MarcaAutenticada != "" {
			marcas[perfil.ID] = regexp.MustCompile(perfil.MarcaAutenticada)
		}
	}
	return marcas
}

// Check informa se a sessão persistida ainda está autenticada. É deliberadamente
// barato: lê os cookies do perfil e faz no máximo uma requisição HTTP. Nenhuma
// página é renderizada e nenhum valor de cookie é registrado em log.
func (m *Manager) Check(ctx context.Context, accountID, plataforma string) (browser.CheckResult, error) {
	perfil := browser.PerfilDe(plataforma)
	result := browser.CheckResult{AccountID: accountID, CheckedAt: time.Now().UTC()}

	cookies, err := m.readCookies(ctx, accountID, plataforma)
	if err != nil {
		return result, err
	}

	found := 0
	for _, cookie := range cookies {
		if browser.IsSessionCookie(perfil.ID, cookie.Name) {
			found++
		}
	}
	if found == 0 {
		result.Reason = fmt.Sprintf("o perfil não possui cookies de sessão do %s", perfil.Label)
		log.Printf("browserd: health check account_id=%s plataforma=%s autenticada=false motivo=sem_cookies",
			accountID, perfil.ID)
		return result, nil
	}

	// Sem uma marca confiável em HTML, os cookies de sessão são o veredito.
	// Um GET sem fingerprint de navegador em Vimeo, Reddit ou X responde com
	// muro de bot ou casca de JavaScript, e ler isso como "deslogado"
	// derrubaria contas boas do rodízio por engano.
	if !perfil.VerificaPorHTTP() {
		result.Authenticated = true
		log.Printf("browserd: health check account_id=%s plataforma=%s autenticada=true por_cookies=%d",
			accountID, perfil.ID, found)
		return result, nil
	}

	client, err := newCheckClient(m.cfg.ProxyURL, cookies)
	if err != nil {
		return result, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, perfil.CheckURL, nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("User-Agent", checkUserAgent)
	request.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")

	response, err := client.Do(request)
	if err != nil {
		return result, fmt.Errorf("não foi possível consultar o %s: %w", perfil.Label, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, checkBodyLimit))
	if err != nil {
		return result, fmt.Errorf("não foi possível ler a resposta do %s: %w", perfil.Label, err)
	}

	finalHost := response.Request.URL.Host
	switch {
	case response.StatusCode >= 500:
		return result, fmt.Errorf("o %s respondeu %d", perfil.Label, response.StatusCode)
	case perfil.HostDeLogin != "" && strings.Contains(finalHost, perfil.HostDeLogin):
		result.Reason = "a sessão foi redirecionada para a tela de login"
	case response.StatusCode != http.StatusOK:
		result.Reason = fmt.Sprintf("o %s respondeu %d", perfil.Label, response.StatusCode)
	case marcasAutenticadas[perfil.ID] != nil && marcasAutenticadas[perfil.ID].Match(body):
		result.Authenticated = true
		if matches := emailPattern.FindSubmatch(body); len(matches) == 2 {
			result.Email = string(matches[1])
		}
	default:
		result.Reason = fmt.Sprintf("o %s não reconheceu a sessão como autenticada", perfil.Label)
	}

	log.Printf("browserd: health check account_id=%s plataforma=%s autenticada=%t cookies_sessao=%d",
		accountID, perfil.ID, result.Authenticated, found)

	return result, nil
}

// newCheckClient monta um cliente com os cookies do perfil. Os cookies são
// agrupados por domínio porque o jar do Go recusa cookies cujo domínio não
// combina com a URL usada no SetCookies.
func newCheckClient(proxyURL string, cookies []cdpCookie) (*http.Client, error) {
	transport := &http.Transport{
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        4,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("proxy configurado é inválido: %w", err)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}

	jar := newCookieJar(cookies)

	return &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   30 * time.Second,
	}, nil
}

// cookieJar é um jar mínimo e somente leitura. Ele evita as regras de aceitação
// do jar padrão, que descartaria cookies de domínios cruzados, e nunca guarda
// nada de volta.
type cookieJar struct {
	byDomain map[string][]*http.Cookie
}

func newCookieJar(cookies []cdpCookie) *cookieJar {
	jar := &cookieJar{byDomain: make(map[string][]*http.Cookie)}

	for _, cookie := range cookies {
		domain := strings.TrimPrefix(strings.ToLower(cookie.Domain), ".")
		jar.byDomain[domain] = append(jar.byDomain[domain], &http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		})
	}
	return jar
}

func (j *cookieJar) SetCookies(_ *url.URL, _ []*http.Cookie) {}

func (j *cookieJar) Cookies(target *url.URL) []*http.Cookie {
	host := strings.ToLower(target.Hostname())

	var result []*http.Cookie
	for domain, cookies := range j.byDomain {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			result = append(result, cookies...)
		}
	}
	return result
}
