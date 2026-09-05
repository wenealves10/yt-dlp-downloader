package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const authHeader = "X-Browser-Token"

// ErrNotConfigured indica que o serviço de navegador não foi configurado neste
// ambiente. A aplicação continua funcionando sem ele, apenas sem contas
// gerenciadas.
var ErrNotConfigured = errors.New("serviço de navegador não configurado")

// ErrSessionUnavailable cobre os casos em que o navegador daquela conta não
// está no ar.
var ErrSessionUnavailable = errors.New("sessão de navegador indisponível")

// ErrServiceUnavailable indica que o serviço de navegador não respondeu. É uma
// falha de infraestrutura, e não um problema com a sessão da conta: quem trata
// o erro não deve concluir que o login expirou.
var ErrServiceUnavailable = errors.New("serviço de navegador indisponível")

// ErrTooManySessions indica que o limite de navegadores simultâneos foi
// atingido. É uma condição operacional: o administrador precisa fechar outro
// navegador antes de abrir este.
var ErrTooManySessions = errors.New("limite de navegadores simultâneos atingido")

// UserMessage traduz os erros do serviço em texto para o painel. O erro
// original continua no log; o que chega à interface não expõe host, porta nem
// caminho interno.
func UserMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrServiceUnavailable):
		return "o serviço de navegador não respondeu"
	case errors.Is(err, ErrTooManySessions):
		return "limite de navegadores simultâneos atingido"
	case errors.Is(err, ErrSessionUnavailable):
		return "o navegador desta conta não está aberto"
	case errors.Is(err, ErrNotConfigured):
		return "serviço de navegador não configurado"
	default:
		return err.Error()
	}
}

// Client conversa com o serviço de navegador remoto. É usado pela API (painel
// do super admin) e pelo worker (obtenção dos cookies para o yt-dlp).
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient devolve nil quando a integração não está configurada, o que permite
// aos chamadores tratarem a ausência do serviço como um estado normal.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || token == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

// Configured permite checar a integração sem espalhar comparações com nil.
func (c *Client) Configured() bool {
	return c != nil
}

func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(authHeader, c.token)

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrServiceUnavailable, err)
	}
	return response, nil
}

func (c *Client) decode(response *http.Response, target any) error {
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		var payload ErrorResponse
		body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))
		_ = json.Unmarshal(body, &payload)

		message := payload.Error
		if message == "" {
			message = fmt.Sprintf("serviço de navegador respondeu %d", response.StatusCode)
		}
		switch response.StatusCode {
		case http.StatusConflict:
			return fmt.Errorf("%w: %s", ErrSessionUnavailable, message)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %s", ErrTooManySessions, message)
		}
		return errors.New(message)
	}

	if target == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(target)
}

// sessionPath monta a rota validando o UUID no lado do chamador também, para
// que um identificador inesperado nunca vire caminho de requisição.
func sessionPath(accountID uuid.UUID, suffix string) string {
	return "/internal/sessions/" + url.PathEscape(accountID.String()) + suffix
}

// EnsureProfile cria o perfil persistente da conta sem abrir o navegador.
func (c *Client) EnsureProfile(ctx context.Context, accountID uuid.UUID) (ProfileInfo, error) {
	var info ProfileInfo
	response, err := c.do(ctx, http.MethodPost, sessionPath(accountID, "/profile"))
	if err != nil {
		return info, err
	}
	return info, c.decode(response, &info)
}

// DeleteProfile encerra a sessão e apaga o perfil em disco.
func (c *Client) DeleteProfile(ctx context.Context, accountID uuid.UUID) error {
	response, err := c.do(ctx, http.MethodDelete, sessionPath(accountID, "/profile"))
	if err != nil {
		return err
	}
	return c.decode(response, nil)
}

// StartSession abre (ou reaproveita) o navegador remoto da conta.
func (c *Client) StartSession(ctx context.Context, accountID uuid.UUID) (SessionInfo, error) {
	var info SessionInfo
	response, err := c.do(ctx, http.MethodPost, sessionPath(accountID, "/start"))
	if err != nil {
		return info, err
	}
	return info, c.decode(response, &info)
}

// StopSession encerra o navegador preservando o perfil.
func (c *Client) StopSession(ctx context.Context, accountID uuid.UUID) (SessionInfo, error) {
	var info SessionInfo
	response, err := c.do(ctx, http.MethodPost, sessionPath(accountID, "/stop"))
	if err != nil {
		return info, err
	}
	return info, c.decode(response, &info)
}

// SessionInfo devolve o estado runtime de uma conta.
func (c *Client) SessionInfo(ctx context.Context, accountID uuid.UUID) (SessionInfo, error) {
	var info SessionInfo
	response, err := c.do(ctx, http.MethodGet, sessionPath(accountID, ""))
	if err != nil {
		return info, err
	}
	return info, c.decode(response, &info)
}

// Sessions devolve o estado de todas as sessões conhecidas pelo serviço, em uma
// única chamada, para montar a listagem do painel.
func (c *Client) Sessions(ctx context.Context) (map[string]SessionInfo, error) {
	response, err := c.do(ctx, http.MethodGet, "/internal/sessions")
	if err != nil {
		return nil, err
	}

	sessions := make(map[string]SessionInfo)
	return sessions, c.decode(response, &sessions)
}

// Check verifica se a sessão persistida continua autenticada.
func (c *Client) Check(ctx context.Context, accountID uuid.UUID) (CheckResult, error) {
	var result CheckResult
	response, err := c.do(ctx, http.MethodPost, sessionPath(accountID, "/check"))
	if err != nil {
		return result, err
	}
	return result, c.decode(response, &result)
}

// Cookies devolve o jar no formato Netscape. O retorno é material sensível:
// nunca deve ser logado, devolvido por API pública ou persistido fora de um
// arquivo temporário com permissão restrita.
func (c *Client) Cookies(ctx context.Context, accountID uuid.UUID) ([]byte, error) {
	response, err := c.do(ctx, http.MethodGet, sessionPath(accountID, "/cookies"))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		var payload ErrorResponse
		body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))
		_ = json.Unmarshal(body, &payload)
		if payload.Error == "" {
			payload.Error = fmt.Sprintf("serviço de navegador respondeu %d", response.StatusCode)
		}
		return nil, errors.New(payload.Error)
	}

	jar, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler a sessão: %w", err)
	}
	if bytes.TrimSpace(jar) == nil {
		return nil, errors.New("a sessão da conta está vazia")
	}
	return jar, nil
}

// DialVNC abre o WebSocket com o servidor gráfico da conta. Quem chama já deve
// ter autorizado o super admin.
func (c *Client) DialVNC(ctx context.Context, accountID uuid.UUID) (*websocket.Conn, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	endpoint, err := url.Parse(c.baseURL + sessionPath(accountID, "/vnc"))
	if err != nil {
		return nil, err
	}
	switch endpoint.Scheme {
	case "https":
		endpoint.Scheme = "wss"
	default:
		endpoint.Scheme = "ws"
	}

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	header := http.Header{}
	header.Set(authHeader, c.token)

	conn, response, err := dialer.DialContext(ctx, endpoint.String(), header)
	if err != nil {
		if response != nil && response.StatusCode == http.StatusConflict {
			return nil, ErrSessionUnavailable
		}
		return nil, fmt.Errorf("não foi possível conectar ao navegador remoto: %w", err)
	}
	return conn, nil
}
