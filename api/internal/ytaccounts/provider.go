// Package ytaccounts é a fronteira entre o gerenciamento de contas do YouTube e
// o mecanismo de download. Os jobs pedem uma sessão e recebem um arquivo de
// cookies temporário; eles nunca sabem que existe um Chrome do outro lado.
package ytaccounts

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// ErrNoAccount indica que não há conta gerenciada autenticada disponível para
// a plataforma. Não é uma falha: o download segue com a configuração estática
// de cookies, que basta para a maioria dos conteúdos públicos.
var ErrNoAccount = errors.New("nenhuma conta autenticada disponível para esta plataforma")

// Lease é o empréstimo de uma sessão autenticada. O arquivo apontado por
// CookieFile existe apenas enquanto o lease estiver aberto.
type Lease struct {
	AccountID uuid.UUID
	Label     string
	// Plataforma é a de onde a sessão veio. Guardada porque um lease do Vimeo
	// devolvido como falho não pode marcar uma conta do YouTube.
	Plataforma string

	// CookieFile é um arquivo 0600 dentro de um diretório temporário 0700,
	// destruído no Release.
	CookieFile string

	once    sync.Once
	tempDir string
}

// Release apaga o arquivo temporário de cookies. Deve ser chamado sempre, de
// preferência via defer, mesmo em caminho de erro.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		if l.tempDir == "" {
			return
		}
		if err := os.RemoveAll(l.tempDir); err != nil {
			log.Printf("ytaccounts: falha ao remover sessão temporária account_id=%s: %v", l.AccountID, err)
		}
	})
}

// Provider entrega sessões autenticadas ao downloader e recebe de volta o
// veredito de quem usou a sessão, para que uma conta reprovada saia do rodízio.
type Provider interface {
	// Acquire pede uma sessão da plataforma do conteúdo. Uma conta do YouTube
	// não autentica no Vimeo, então a plataforma faz parte do pedido.
	Acquire(ctx context.Context, plataforma string) (*Lease, error)
	ReportAuthFailure(ctx context.Context, lease *Lease, reason string)
}

// Manager implementa Provider sobre o banco e o serviço de navegador.
type Manager struct {
	store  db.Store
	client *browser.Client
}

func NewManager(store db.Store, client *browser.Client) *Manager {
	return &Manager{store: store, client: client}
}

// maxAcquireAttempts limita quantas contas são tentadas em uma única
// requisição. Sem o teto, um problema geral (o serviço de navegador fora do ar)
// faria o downloader percorrer todas as contas antes de desistir.
const maxAcquireAttempts = 5

// Acquire escolhe uma conta autenticada e materializa o jar de cookies em
// disco. As contas entram em rodízio pelo uso mais antigo, respeitando a
// prioridade, e a seleção acontece de forma atômica no banco — dois workers
// simultâneos pegam contas diferentes.
//
// Quando a sessão escolhida se revela inválida, a conta é marcada e a próxima
// é tentada na mesma chamada, de modo que um cookie expirado não derruba o
// download enquanto houver outra conta funcionando.
func (m *Manager) Acquire(ctx context.Context, plataforma string) (*Lease, error) {
	if m == nil || m.store == nil || !m.client.Configured() {
		return nil, ErrNoAccount
	}

	// Uma plataforma sem perfil de login não tem como ter conta gerenciada;
	// procurar no banco só gastaria uma consulta para achar nada.
	if !browser.PlataformaSuportada(plataforma) {
		return nil, ErrNoAccount
	}

	tried := make([]uuid.UUID, 0, maxAcquireAttempts)

	for attempt := 0; attempt < maxAcquireAttempts; attempt++ {
		account, err := m.store.ClaimYoutubeAccount(ctx, db.ClaimYoutubeAccountParams{
			TargetPlatform: plataforma,
			Exclude:        tried,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Sem mais contas elegíveis. Se alguma foi tentada e falhou, o
				// erro dela é mais informativo do que "nenhuma conta".
				if len(tried) > 0 {
					return nil, fmt.Errorf("nenhuma conta de %s utilizável: %d tentada(s) sem sucesso",
						browser.PerfilDe(plataforma).Label, len(tried))
				}
				return nil, ErrNoAccount
			}
			return nil, fmt.Errorf("não foi possível selecionar uma conta: %w", err)
		}
		tried = append(tried, account.ID)

		lease, err := m.leaseFor(ctx, account)
		if err != nil {
			log.Printf("ytaccounts: conta descartada nesta tentativa account_id=%s label=%s motivo=%v",
				account.ID, account.Label, err)
			continue
		}

		log.Printf("ytaccounts: download usando conta account_id=%s label=%s tentativa=%d",
			account.ID, account.Label, attempt+1)
		return lease, nil
	}

	return nil, fmt.Errorf("nenhuma conta de %s utilizável após %d tentativas",
		browser.PerfilDe(plataforma).Label, maxAcquireAttempts)
}

// leaseFor busca os cookies da conta e grava o arquivo temporário. Uma sessão
// sem os cookies do Google é rejeitada aqui, sem gastar uma requisição ao
// YouTube: é o caso mais comum de conta deslogada.
func (m *Manager) leaseFor(ctx context.Context, account db.YoutubeAccount) (*Lease, error) {
	perfil := browser.PerfilDe(account.Platform)

	jar, err := m.client.Cookies(ctx, account.ID, account.Platform)
	if err != nil {
		m.markUnavailable(ctx, account, err)
		return nil, err
	}

	if browser.CountSessionCookies(account.Platform, jar) == 0 {
		reason := fmt.Errorf("o perfil não possui cookies de sessão do %s", perfil.Label)
		m.markUnavailable(ctx, account, reason)
		return nil, reason
	}

	tempDir, err := os.MkdirTemp("", "ytsession-")
	if err != nil {
		return nil, fmt.Errorf("não foi possível preparar a sessão: %w", err)
	}
	if err := os.Chmod(tempDir, 0o700); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("não foi possível proteger a sessão: %w", err)
	}

	cookieFile := filepath.Join(tempDir, "cookies.txt")
	if err := os.WriteFile(cookieFile, jar, 0o600); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("não foi possível gravar a sessão: %w", err)
	}

	return &Lease{
		AccountID:  account.ID,
		Label:      account.Label,
		Plataforma: account.Platform,
		CookieFile: cookieFile,
		tempDir:    tempDir,
	}, nil
}

// ReportAuthFailure marca a conta do lease como precisando de novo login. É
// chamado quando o yt-dlp deixa claro que a sessão não vale mais; a partir daí
// o rodízio deixa de escolher essa conta até o health check ou o administrador
// a recuperarem.
func (m *Manager) ReportAuthFailure(ctx context.Context, lease *Lease, reason string) {
	if m == nil || m.store == nil || lease == nil {
		return
	}

	log.Printf("ytaccounts: sessão rejeitada pela plataforma account_id=%s label=%s plataforma=%s",
		lease.AccountID, lease.Label, lease.Plataforma)

	if _, err := m.store.UpdateYoutubeAccountStatus(ctx, db.UpdateYoutubeAccountStatusParams{
		ID:        lease.AccountID,
		Status:    db.CoreYoutubeAccountStatusREQUIRESAUTH,
		LastError: pgtype.Text{String: truncateError(reason), Valid: reason != ""},
	}); err != nil {
		log.Printf("ytaccounts: falha ao marcar conta como REQUIRES_AUTH account_id=%s: %v", lease.AccountID, err)
	}
}

func (m *Manager) markUnavailable(ctx context.Context, account db.YoutubeAccount, cause error) {
	status := db.CoreYoutubeAccountStatusREQUIRESAUTH
	if errors.Is(cause, browser.ErrServiceUnavailable) {
		status = db.CoreYoutubeAccountStatusERROR
	}

	log.Printf("ytaccounts: conta indisponível account_id=%s label=%s status=%s erro=%v",
		account.ID, account.Label, status, cause)

	if _, err := m.store.UpdateYoutubeAccountStatus(ctx, db.UpdateYoutubeAccountStatusParams{
		ID:        account.ID,
		Status:    status,
		LastError: pgtype.Text{String: truncateError(browser.UserMessage(cause)), Valid: true},
	}); err != nil {
		log.Printf("ytaccounts: falha ao gravar status da conta account_id=%s: %v", account.ID, err)
	}
}

// truncateError evita que uma mensagem gigante ocupe a coluna de erro.
func truncateError(message string) string {
	const limit = 500
	if len(message) <= limit {
		return message
	}
	return message[:limit] + "…"
}

// AuthArgs monta os argumentos de autenticação do yt-dlp. Com uma conta
// gerenciada, usa a sessão emprestada; sem ela, mantém o arquivo estático já
// configurado. O proxy, quando habilitado, é somado em vez de substituir os
// cookies — as duas coisas são independentes.
func AuthArgs(lease *Lease) []string {
	cfg := configs.LoadedConfig
	args := make([]string, 0, 10)

	if cfg.ProxyEnabled && cfg.ProxyURL != "" {
		args = append(args, "--proxy", cfg.ProxyURL)
	}

	switch {
	case lease != nil && lease.CookieFile != "":
		args = append(args, "--cookies", lease.CookieFile)
	case cfg.YoutubeDLFileCookies != "":
		args = append(args, "--cookies", cfg.YoutubeDLFileCookies)
	}

	if cfg.YoutubeDLUserAgent != "" {
		args = append(args, "--user-agent", cfg.YoutubeDLUserAgent)
	}
	if cfg.YoutubeDLReferer != "" {
		args = append(args, "--referer", cfg.YoutubeDLReferer)
	}
	args = append(args, "--add-header", "DNT: 1")

	return args
}

// Acquire é o atalho usado pelos jobs: devolve o lease quando existe uma conta
// autenticada e nil quando não existe, sem transformar a ausência em erro.
func Acquire(ctx context.Context, provider Provider, plataforma string) *Lease {
	if provider == nil {
		return nil
	}

	lease, err := provider.Acquire(ctx, plataforma)
	if err != nil {
		if !errors.Is(err, ErrNoAccount) {
			log.Printf("ytaccounts: seguindo sem conta gerenciada: %v", err)
		}
		return nil
	}
	return lease
}

// authFailureMarkers são as mensagens do yt-dlp que indicam, sem ambiguidade,
// que a sessão não foi aceita. Restrições de conteúdo (vídeo privado, só para
// membros, bloqueio regional) ficam de fora de propósito: elas não significam
// que a conta parou de funcionar e não devem tirá-la do rodízio.
//
// "The page needs to be reloaded" também fica de fora: essa mensagem aparece
// quando falta o runtime JavaScript, e não quando a sessão expirou.
var authFailureMarkers = []string{
	"cookies are no longer valid",
	"account cookies are invalid",
	"sign in to confirm you're not a bot",
	"sign in to confirm your age",
	"please sign in",
	"--cookies for the authentication",
	// As outras plataformas dizem a mesma coisa com outras palavras. Sem
	// reconhecê-las, uma sessão expirada do Vimeo ficaria no rodízio para
	// sempre, falhando todo download que a pegasse.
	"only works when logged-in",
	"login required",
	"you must be logged in",
	"requires authentication",
	"log in to view",
}

// causasTransitorias são falhas que NÃO são culpa da conta: a plataforma
// recusando o IP do servidor, pedindo uma pausa, ou a rede caindo.
//
// Elas têm precedência sobre qualquer marcador de autenticação, porque várias
// plataformas misturam as duas ideias na mesma mensagem — o X responde a um IP
// bloqueado com um texto que também fala em login. Sem esta lista, um bloqueio
// temporário tirava do rodízio uma conta perfeitamente boa, e a tentativa
// seguinte ia sem cookie nenhum: o primeiro tropeço envenenava todas as
// próximas, e o administrador via a conta "desconectando" sozinha.
var causasTransitorias = []string{
	"http error 403", "http error 429", "http error 5",
	"rate limit", "rate-limit", "too many requests",
	"blocked", "access denied", "forbidden", "captcha",
	"connection reset", "connection refused", "network is unreachable",
	"temporary failure in name resolution", "read operation timed out",
	"connection timed out", "unable to connect to proxy",
}

// ehTransitorio informa se a saída aponta para uma causa que não é da conta.
func ehTransitorio(minuscula string) bool {
	for _, marcador := range causasTransitorias {
		if strings.Contains(minuscula, marcador) {
			return true
		}
	}
	return false
}

// IsAuthFailure informa se a saída do yt-dlp indica sessão recusada.
func IsAuthFailure(output []byte) bool {
	return authFailureLine(output) != ""
}

// AuthFailureReason devolve a linha que motivou o veredito, para o painel
// mostrar ao administrador por que a conta saiu do rodízio.
func AuthFailureReason(output []byte) string {
	line := authFailureLine(output)
	if line == "" {
		return ""
	}
	return "a plataforma recusou a sessão: " + line
}

func authFailureLine(output []byte) string {
	// Causa transitória vence: a conta não tem culpa e precisa continuar no
	// rodízio para a próxima tentativa ainda ter uma sessão para usar.
	if ehTransitorio(strings.ToLower(string(output))) {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		lowered := strings.ToLower(trimmed)
		for _, marker := range authFailureMarkers {
			if strings.Contains(lowered, marker) {
				return truncateError(trimmed)
			}
		}
	}
	return ""
}

// ReportAuthFailure é o atalho usado pelos jobs: não faz nada quando não havia
// conta gerenciada envolvida.
func ReportAuthFailure(ctx context.Context, provider Provider, lease *Lease, output []byte) {
	if provider == nil || lease == nil {
		return
	}

	// A GUARDA que faltava. Antes daqui, QUALQUER falha de download tirava a
	// conta do rodízio — bloqueio de IP, limite de requisições, queda de rede,
	// disco cheio. A conta ia para "Requer autenticação" com a sessão intacta,
	// o administrador clicava em "Verificar sessão" e ela voltava, porque nunca
	// tinha deixado de valer. É o que fazia a conta parecer que desconectava
	// sozinha.
	motivo := AuthFailureReason(output)
	if motivo == "" {
		log.Printf("ytaccounts: falha sem relação com a sessão; conta mantida no rodízio account_id=%s plataforma=%s",
			lease.AccountID, lease.Plataforma)
		return
	}

	provider.ReportAuthFailure(ctx, lease, motivo)
}
