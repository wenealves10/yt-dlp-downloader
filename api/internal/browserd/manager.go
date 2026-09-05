// Package browserd implementa o serviço de navegador remoto. Ele é o único
// componente do sistema que conhece Chrome, Xvfb e VNC: a API e o worker falam
// apenas o contrato HTTP definido em internal/browser.
package browserd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
)

const (
	// Faixa de displays X reservada para as sessões. Cada conta recebe um
	// display próprio, o que garante que uma sessão nunca enxergue a janela da
	// outra.
	firstDisplay = 99
	lastDisplay  = 148

	startupTimeout  = 45 * time.Second
	shutdownTimeout = 15 * time.Second
)

// ErrSessionNotRunning é devolvido quando alguém tenta usar uma sessão que não
// está no ar (por exemplo, conectar o VNC depois de o navegador ser encerrado).
var ErrSessionNotRunning = errors.New("sessão de navegador não está ativa")

// ErrTooManySessions protege a memória do host: navegadores headful são caros.
var ErrTooManySessions = errors.New("limite de navegadores simultâneos atingido")

// ErrInvalidAccountID barra qualquer identificador que não seja um UUID, o que
// elimina path traversal na montagem do caminho do perfil.
var ErrInvalidAccountID = errors.New("identificador de conta inválido")

// Config reúne os parâmetros operacionais do serviço.
type Config struct {
	ProfilesDir    string
	ChromeBinary   string
	IdleTimeout    time.Duration
	Width          int
	Height         int
	MaxSessions    int
	DisableSandbox bool
	ProxyURL       string
	UserAgent      string
}

type session struct {
	accountID    string
	display      int
	vncPort      int
	cdpPort      int
	vncPassword  string
	width        int
	height       int
	state        browser.SessionState
	startedAt    time.Time
	lastActivity time.Time
	viewers      int
	lastErr      string

	xvfb   *proc
	wm     *proc
	chrome *proc
	vnc    *proc
}

// proc é um processo filho supervisionado. Um único goroutine faz Wait, para
// que encerramento e supervisão nunca disputem a colheita do mesmo filho.
type proc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func startProc(cmd *exec.Cmd) (*proc, error) {
	// Setpgid isola o grupo de processos: se o encerramento gracioso falhar,
	// matamos o grupo inteiro sem atingir o serviço.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	p := &proc{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// terminate pede o encerramento gracioso e só depois mata o grupo de processos.
// O Chrome precisa do SIGTERM para gravar os cookies em disco antes de sair.
func (p *proc) terminate(grace time.Duration) {
	if p == nil || p.cmd.Process == nil {
		return
	}

	select {
	case <-p.done:
		return
	default:
	}

	_ = p.cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-p.done:
	case <-time.After(grace):
		if pgid, err := syscall.Getpgid(p.cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = p.cmd.Process.Kill()
		}
		<-p.done
	}
}

// Manager controla o ciclo de vida dos navegadores e dos perfis persistentes.
type Manager struct {
	cfg Config

	mu       sync.Mutex
	sessions map[string]*session

	// locks serializa as operações por conta. Um perfil do Chrome só suporta
	// um processo por vez, então abrir, fechar, exportar cookies e verificar a
	// sessão nunca podem correr em paralelo para a mesma conta.
	locks sync.Map

	stop chan struct{}
	wg   sync.WaitGroup
}

// NewManager prepara o diretório de perfis e limpa resíduos de execuções
// anteriores (locks de perfil e de display deixados por um container morto).
func NewManager(cfg Config) (*Manager, error) {
	if cfg.ProfilesDir == "" {
		return nil, errors.New("diretório de perfis não configurado")
	}
	if cfg.ChromeBinary == "" {
		cfg.ChromeBinary = "google-chrome"
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 15 * time.Minute
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		cfg.Width, cfg.Height = 1440, 900
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 2
	}

	// 0700: apenas o usuário do serviço enxerga os perfis.
	if err := os.MkdirAll(cfg.ProfilesDir, 0o700); err != nil {
		return nil, fmt.Errorf("não foi possível criar o diretório de perfis: %w", err)
	}
	if err := os.Chmod(cfg.ProfilesDir, 0o700); err != nil {
		log.Printf("browserd: não foi possível ajustar permissões de %s: %v", cfg.ProfilesDir, err)
	}

	m := &Manager{
		cfg:      cfg,
		sessions: make(map[string]*session),
		stop:     make(chan struct{}),
	}

	m.recoverFromRestart()

	m.wg.Add(1)
	go m.reapIdleSessions()

	return m, nil
}

// Close encerra todas as sessões ativas. O perfil persistente permanece.
func (m *Manager) Close() {
	close(m.stop)
	m.wg.Wait()

	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		if err := m.Stop(id); err != nil {
			log.Printf("browserd: falha ao encerrar sessão account_id=%s: %v", id, err)
		}
	}
}

// recoverFromRestart remove locks órfãos. Quando o container morre, o Chrome
// deixa para trás SingletonLock apontando para um PID que não existe mais e o
// Xvfb deixa /tmp/.X<n>-lock; sem essa limpeza o navegador se recusaria a abrir
// de novo.
func (m *Manager) recoverFromRestart() {
	entries, err := os.ReadDir(m.cfg.ProfilesDir)
	if err != nil {
		log.Printf("browserd: não foi possível inspecionar perfis: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := uuid.Parse(entry.Name()); err != nil {
			continue
		}
		m.clearProfileLocks(filepath.Join(m.cfg.ProfilesDir, entry.Name()))
	}

	for display := firstDisplay; display <= lastDisplay; display++ {
		lock := fmt.Sprintf("/tmp/.X%d-lock", display)
		data, err := os.ReadFile(lock)
		if err != nil {
			continue
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if convErr == nil && processAlive(pid) {
			continue
		}
		if err := os.Remove(lock); err == nil {
			log.Printf("browserd: lock de display órfão removido display=:%d", display)
		}
		_ = os.Remove(fmt.Sprintf("/tmp/.X11-unix/X%d", display))
	}
}

func (m *Manager) clearProfileLocks(profileDir string) {
	for _, name := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
		path := filepath.Join(profileDir, name)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			log.Printf("browserd: não foi possível remover %s: %v", name, err)
			continue
		}
		log.Printf("browserd: lock de perfil órfão removido arquivo=%s", name)
	}
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// lockAccount serializa as operações de uma conta.
func (m *Manager) lockAccount(accountID string) func() {
	value, _ := m.locks.LoadOrStore(accountID, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ProfilePath monta o caminho do perfil validando o identificador. Só um UUID
// é aceito, e o resultado é conferido contra o diretório raiz.
func (m *Manager) ProfilePath(accountID string) (string, error) {
	parsed, err := uuid.Parse(accountID)
	if err != nil || parsed == uuid.Nil {
		return "", ErrInvalidAccountID
	}

	root, err := filepath.Abs(m.cfg.ProfilesDir)
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, parsed.String())
	if filepath.Dir(path) != root {
		return "", ErrInvalidAccountID
	}
	return path, nil
}

// EnsureProfile cria o diretório do perfil, sem abrir o navegador. É o passo
// "preparar a sessão" executado quando a conta é cadastrada.
func (m *Manager) EnsureProfile(accountID string) (browser.ProfileInfo, error) {
	path, err := m.ProfilePath(accountID)
	if err != nil {
		return browser.ProfileInfo{}, err
	}

	unlock := m.lockAccount(accountID)
	defer unlock()

	if err := os.MkdirAll(path, 0o700); err != nil {
		return browser.ProfileInfo{}, fmt.Errorf("não foi possível criar o perfil: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return browser.ProfileInfo{}, fmt.Errorf("não foi possível ajustar permissões do perfil: %w", err)
	}

	return m.profileInfo(accountID, path), nil
}

// DeleteProfile encerra a sessão e apaga definitivamente os dados do perfil.
func (m *Manager) DeleteProfile(accountID string) error {
	path, err := m.ProfilePath(accountID)
	if err != nil {
		return err
	}

	if err := m.Stop(accountID); err != nil && !errors.Is(err, ErrSessionNotRunning) {
		return err
	}

	unlock := m.lockAccount(accountID)
	defer unlock()

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("não foi possível remover o perfil: %w", err)
	}
	log.Printf("browserd: perfil removido account_id=%s", accountID)
	return nil
}

// Profile devolve o estado do perfil em disco.
func (m *Manager) Profile(accountID string) (browser.ProfileInfo, error) {
	path, err := m.ProfilePath(accountID)
	if err != nil {
		return browser.ProfileInfo{}, err
	}
	return m.profileInfo(accountID, path), nil
}

func (m *Manager) profileInfo(accountID, path string) browser.ProfileInfo {
	info := browser.ProfileInfo{AccountID: accountID}
	stat, err := os.Stat(path)
	if err != nil || !stat.IsDir() {
		return info
	}
	info.Exists = true

	var size int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if fileInfo, err := entry.Info(); err == nil {
			size += fileInfo.Size()
		}
		return nil
	})
	info.SizeBytes = size
	return info
}

// Info devolve o estado runtime da sessão. Uma conta sem navegador aberto
// aparece como STOPPED, e não como erro.
func (m *Manager) Info(accountID string) (browser.SessionInfo, error) {
	if _, err := m.ProfilePath(accountID); err != nil {
		return browser.SessionInfo{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.sessions[accountID]
	if !ok {
		return browser.SessionInfo{AccountID: accountID, State: browser.SessionStateStopped}, nil
	}
	return current.info(), nil
}

// List devolve o estado de todas as sessões conhecidas pelo serviço.
func (m *Manager) List() map[string]browser.SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]browser.SessionInfo, len(m.sessions))
	for id, current := range m.sessions {
		result[id] = current.info()
	}
	return result
}

func (s *session) info() browser.SessionInfo {
	info := browser.SessionInfo{
		AccountID:   s.accountID,
		State:       s.state,
		Viewers:     s.viewers,
		Width:       s.width,
		Height:      s.height,
		VNCPassword: s.vncPassword,
		Error:       s.lastErr,
	}
	if !s.startedAt.IsZero() {
		startedAt := s.startedAt
		info.StartedAt = &startedAt
	}
	if !s.lastActivity.IsZero() {
		lastActivity := s.lastActivity
		info.LastActivity = &lastActivity
	}
	return info
}

// Start abre (ou reaproveita) o navegador da conta. É idempotente: dois pedidos
// simultâneos devolvem a mesma sessão, nunca dois navegadores sobre o mesmo
// perfil.
func (m *Manager) Start(accountID string) (browser.SessionInfo, error) {
	profileDir, err := m.ProfilePath(accountID)
	if err != nil {
		return browser.SessionInfo{}, err
	}

	unlock := m.lockAccount(accountID)
	defer unlock()

	m.mu.Lock()
	if current, ok := m.sessions[accountID]; ok && current.state == browser.SessionStateRunning {
		current.lastActivity = time.Now()
		info := current.info()
		m.mu.Unlock()
		return info, nil
	}
	if len(m.sessions) >= m.cfg.MaxSessions {
		if _, ok := m.sessions[accountID]; !ok {
			m.mu.Unlock()
			return browser.SessionInfo{}, ErrTooManySessions
		}
	}
	m.mu.Unlock()

	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return browser.SessionInfo{}, fmt.Errorf("não foi possível criar o perfil: %w", err)
	}
	m.clearProfileLocks(profileDir)

	created, err := m.launch(accountID, profileDir)
	if err != nil {
		m.mu.Lock()
		m.sessions[accountID] = &session{
			accountID: accountID,
			state:     browser.SessionStateError,
			lastErr:   err.Error(),
			width:     m.cfg.Width,
			height:    m.cfg.Height,
		}
		m.mu.Unlock()
		log.Printf("browserd: falha ao abrir navegador account_id=%s erro=%v", accountID, err)
		return browser.SessionInfo{}, err
	}

	m.mu.Lock()
	m.sessions[accountID] = created
	info := created.info()
	m.mu.Unlock()

	log.Printf("browserd: navegador iniciado account_id=%s display=:%d", accountID, created.display)
	go m.watchChrome(created)

	return info, nil
}

// Stop encerra o navegador preservando o perfil. Chrome recebe SIGTERM para ter
// a chance de gravar os cookies em disco antes de morrer.
func (m *Manager) Stop(accountID string) error {
	if _, err := m.ProfilePath(accountID); err != nil {
		return err
	}

	unlock := m.lockAccount(accountID)
	defer unlock()

	m.mu.Lock()
	current, ok := m.sessions[accountID]
	if !ok {
		m.mu.Unlock()
		return ErrSessionNotRunning
	}
	delete(m.sessions, accountID)
	m.mu.Unlock()

	m.teardown(current)
	log.Printf("browserd: navegador encerrado account_id=%s", accountID)
	return nil
}

// watchChrome reage ao navegador fechado pelo próprio administrador (ou a um
// crash): derruba Xvfb, x11vnc e o gerenciador de janelas para não deixar
// processos zumbis consumindo memória.
func (m *Manager) watchChrome(s *session) {
	if s.chrome == nil {
		return
	}
	<-s.chrome.done

	m.mu.Lock()
	current, ok := m.sessions[s.accountID]
	if !ok || current != s {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, s.accountID)
	m.mu.Unlock()

	log.Printf("browserd: navegador terminou sozinho account_id=%s", s.accountID)
	m.teardownSupport(s)
}

func (m *Manager) reapIdleSessions() {
	defer m.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			now := time.Now()
			var idle []string

			m.mu.Lock()
			for id, current := range m.sessions {
				if current.viewers > 0 {
					current.lastActivity = now
					continue
				}
				if now.Sub(current.lastActivity) > m.cfg.IdleTimeout {
					idle = append(idle, id)
				}
			}
			m.mu.Unlock()

			for _, id := range idle {
				log.Printf("browserd: encerrando navegador ocioso account_id=%s", id)
				if err := m.Stop(id); err != nil && !errors.Is(err, ErrSessionNotRunning) {
					log.Printf("browserd: falha ao encerrar navegador ocioso account_id=%s: %v", id, err)
				}
			}
		}
	}
}

// AddViewer marca que existe um administrador conectado ao VNC, o que impede o
// reaper de fechar o navegador durante o login.
func (m *Manager) AddViewer(accountID string) (int, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.sessions[accountID]
	if !ok || current.state != browser.SessionStateRunning {
		return 0, "", ErrSessionNotRunning
	}
	current.viewers++
	current.lastActivity = time.Now()
	return current.vncPort, current.vncPassword, nil
}

// RemoveViewer desfaz AddViewer.
func (m *Manager) RemoveViewer(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.sessions[accountID]
	if !ok {
		return
	}
	if current.viewers > 0 {
		current.viewers--
	}
	current.lastActivity = time.Now()
}

// runningCDPPort informa a porta do DevTools quando já existe um navegador no
// ar para a conta. É assim que a extração de cookies evita subir um segundo
// Chrome sobre um perfil ocupado.
func (m *Manager) runningCDPPort(accountID string) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.sessions[accountID]
	if !ok || current.state != browser.SessionStateRunning {
		return 0, false
	}
	return current.cdpPort, true
}

func (m *Manager) teardown(s *session) {
	s.chrome.terminate(shutdownTimeout)
	m.teardownSupport(s)
}

func (m *Manager) teardownSupport(s *session) {
	s.wm.terminate(5 * time.Second)
	s.vnc.terminate(5 * time.Second)
	s.xvfb.terminate(5 * time.Second)
	_ = os.Remove(fmt.Sprintf("/tmp/.X%d-lock", s.display))
	_ = os.Remove(fmt.Sprintf("/tmp/.X11-unix/X%d", s.display))
}

// freePort reserva uma porta efêmera em 127.0.0.1. Nenhuma porta do serviço é
// publicada para fora do container.
func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("porta %d não respondeu em %s", port, timeout)
}
