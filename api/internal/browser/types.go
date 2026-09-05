// Package browser define o contrato entre a aplicação (API e worker) e o
// serviço de navegador remoto. Só trafegam metadados de sessão por aqui: o
// perfil persistente do Chrome, com os cookies de autenticação, nunca sai do
// container do serviço, exceto pelo jar temporário entregue ao downloader.
package browser

import "time"

type SessionState string

const (
	// SessionStateStopped indica que só existe o perfil persistente em disco.
	SessionStateStopped SessionState = "STOPPED"
	// SessionStateStarting cobre a janela entre o pedido e o navegador pronto.
	SessionStateStarting SessionState = "STARTING"
	// SessionStateRunning indica navegador e servidor VNC no ar.
	SessionStateRunning SessionState = "RUNNING"
	// SessionStateError indica falha ao subir ou queda inesperada.
	SessionStateError SessionState = "ERROR"
)

// SessionInfo é o estado runtime de uma sessão de navegador. Não é persistido
// no banco: uma reinicialização do serviço zera tudo naturalmente.
type SessionInfo struct {
	AccountID    string       `json:"account_id"`
	State        SessionState `json:"state"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	LastActivity *time.Time   `json:"last_activity,omitempty"`
	Viewers      int          `json:"viewers"`
	Width        int          `json:"width"`
	Height       int          `json:"height"`
	// VNCPassword protege o servidor RFB, que já escuta apenas em 127.0.0.1
	// dentro do container. Só é devolvido ao super admin autenticado, para que
	// o cliente noVNC consiga completar o handshake.
	VNCPassword string `json:"vnc_password,omitempty"`
	Error       string `json:"error,omitempty"`
}

// CheckResult é o veredito do health check de uma sessão.
type CheckResult struct {
	AccountID     string    `json:"account_id"`
	Authenticated bool      `json:"authenticated"`
	Email         string    `json:"email,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
}

// ProfileInfo descreve o perfil persistente em disco.
type ProfileInfo struct {
	AccountID string `json:"account_id"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes"`
}

// ErrorResponse é o corpo devolvido pelo serviço em qualquer falha.
type ErrorResponse struct {
	Error string `json:"error"`
}
