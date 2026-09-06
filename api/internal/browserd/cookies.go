package browserd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
)

// Os domínios exportáveis vêm de browser.PerfilDe: cada plataforma libera só o
// que o downloader precisa dela. Cookies de qualquer outro site que o
// administrador tenha visitado no navegador remoto nunca saem do container —
// inclusive os de OUTRA plataforma gerenciada aqui, que não têm por que vazar
// para o download de uma terceira.

// cdpCookie espelha o formato do Chrome DevTools Protocol.
type cdpCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	Session  bool    `json:"session"`
}

type cdpMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Cookies devolve o jar de cookies da conta no formato Netscape, pronto para o
// yt-dlp. Se o navegador já estiver aberto reutiliza a instância; caso
// contrário sobe um Chrome headless efêmero sobre o mesmo perfil, lê os cookies
// pelo DevTools e o encerra. O chamador deve tratar o retorno como segredo.
func (m *Manager) Cookies(ctx context.Context, accountID, plataforma string) ([]byte, error) {
	cookies, err := m.readCookies(ctx, accountID, plataforma)
	if err != nil {
		return nil, err
	}
	return formatNetscape(cookies), nil
}

func (m *Manager) readCookies(ctx context.Context, accountID, plataforma string) ([]cdpCookie, error) {
	profileDir, err := m.ProfilePath(accountID)
	if err != nil {
		return nil, err
	}

	if port, running := m.runningCDPPort(accountID); running {
		return fetchCookiesFromCDP(ctx, port, plataforma)
	}

	unlock := m.lockAccount(accountID)
	defer unlock()

	// A sessão pode ter sido aberta entre a checagem acima e o lock.
	if port, running := m.runningCDPPort(accountID); running {
		return fetchCookiesFromCDP(ctx, port, plataforma)
	}

	if stat, err := os.Stat(profileDir); err != nil || !stat.IsDir() {
		return nil, fmt.Errorf("perfil da conta ainda não foi criado")
	}
	m.clearProfileLocks(profileDir)

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("não foi possível reservar porta do DevTools: %w", err)
	}

	args := []string{
		"--headless=new",
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--password-store=basic",
		"--use-mock-keychain",
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
		"about:blank",
	}
	if m.cfg.DisableSandbox {
		args = append([]string{"--no-sandbox"}, args...)
	}

	cmd := exec.Command(m.cfg.ChromeBinary, args...)
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS=/dev/null")
	cmd.Stdout, cmd.Stderr = logWriter(accountID, "chrome-headless"), logWriter(accountID, "chrome-headless")

	headless, err := startProc(cmd)
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir o perfil: %w", err)
	}
	defer headless.terminate(shutdownTimeout)

	if err := waitForPort(ctx, port, 30*time.Second); err != nil {
		return nil, fmt.Errorf("o perfil não ficou disponível: %w", err)
	}

	return fetchCookiesFromCDP(ctx, port, plataforma)
}

// fetchCookiesFromCDP conversa com o endpoint de browser do DevTools. Storage
// .getCookies devolve o contexto padrão inteiro, sem precisar navegar.
func fetchCookiesFromCDP(ctx context.Context, port int, plataforma string) ([]cdpCookie, error) {
	endpoint, err := devToolsWebSocketURL(ctx, port)
	if err != nil {
		return nil, err
	}

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("não foi possível conectar ao DevTools: %w", err)
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(20 * time.Second)
	}
	_ = conn.SetWriteDeadline(deadline)
	_ = conn.SetReadDeadline(deadline)

	if err := conn.WriteJSON(map[string]any{"id": 1, "method": "Storage.getCookies"}); err != nil {
		return nil, fmt.Errorf("não foi possível consultar os cookies: %w", err)
	}

	for {
		var message cdpMessage
		if err := conn.ReadJSON(&message); err != nil {
			return nil, fmt.Errorf("não foi possível ler a resposta do DevTools: %w", err)
		}
		if message.ID != 1 {
			continue
		}
		if message.Error != nil {
			return nil, fmt.Errorf("DevTools recusou a consulta: %s", message.Error.Message)
		}

		var payload struct {
			Cookies []cdpCookie `json:"cookies"`
		}
		if err := json.Unmarshal(message.Result, &payload); err != nil {
			return nil, fmt.Errorf("resposta do DevTools inválida: %w", err)
		}
		return filterCookies(payload.Cookies, plataforma), nil
	}
}

func devToolsWebSocketURL(ctx context.Context, port int) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return "", fmt.Errorf("DevTools indisponível: %w", err)
	}
	defer response.Body.Close()

	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("resposta do DevTools inválida: %w", err)
	}
	if payload.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("DevTools não expôs o endpoint de depuração")
	}
	return payload.WebSocketDebuggerURL, nil
}

func filterCookies(cookies []cdpCookie, plataforma string) []cdpCookie {
	permitidos := browser.PerfilDe(plataforma).CookieDomains

	filtered := make([]cdpCookie, 0, len(cookies))
	for _, cookie := range cookies {
		if matchesCookieDomain(cookie.Domain, permitidos) {
			filtered = append(filtered, cookie)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Domain != filtered[j].Domain {
			return filtered[i].Domain < filtered[j].Domain
		}
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

func matchesCookieDomain(domain string, permitidos []string) bool {
	host := strings.TrimPrefix(strings.ToLower(domain), ".")
	for _, allowed := range permitidos {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// formatNetscape serializa o jar no formato que o yt-dlp espera em --cookies.
func formatNetscape(cookies []cdpCookie) []byte {
	var builder strings.Builder
	builder.WriteString("# Netscape HTTP Cookie File\n")
	builder.WriteString("# Gerado pelo advideo-browser. Nao editar manualmente.\n\n")

	for _, cookie := range cookies {
		includeSubdomains := "FALSE"
		if strings.HasPrefix(cookie.Domain, ".") {
			includeSubdomains = "TRUE"
		}

		secure := "FALSE"
		if cookie.Secure {
			secure = "TRUE"
		}

		path := cookie.Path
		if path == "" {
			path = "/"
		}

		var expires int64
		if !cookie.Session && cookie.Expires > 0 {
			expires = int64(cookie.Expires)
		}

		fmt.Fprintf(&builder, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			cookie.Domain, includeSubdomains, path, secure, expires, cookie.Name, cookie.Value)
	}

	return []byte(builder.String())
}
