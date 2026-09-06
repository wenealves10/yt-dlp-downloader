package browserd

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
)

// startURL abre direto na tela de login do Google já apontando o retorno para o
// YouTube. O administrador digita as credenciais aqui; o sistema nunca as vê.
const startURL = "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Fwww.youtube.com%2F"

// vncPasswordAlphabet evita caracteres ambíguos. O protocolo RFB só considera
// os 8 primeiros bytes da senha, por isso o tamanho fixo.
const (
	vncPasswordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	vncPasswordLength   = 8
)

// launch sobe a pilha gráfica da sessão: Xvfb (tela virtual), openbox (foco e
// decoração de janelas, necessários para os popups do login do Google), Chrome
// headful e x11vnc.
func (m *Manager) launch(accountID, profileDir string) (*session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	display, err := m.allocateDisplay()
	if err != nil {
		return nil, err
	}

	vncPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("não foi possível reservar porta do VNC: %w", err)
	}
	cdpPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("não foi possível reservar porta do DevTools: %w", err)
	}
	password, err := randomVNCPassword()
	if err != nil {
		return nil, fmt.Errorf("não foi possível gerar a senha do VNC: %w", err)
	}

	created := &session{
		accountID:    accountID,
		display:      display,
		vncPort:      vncPort,
		cdpPort:      cdpPort,
		vncPassword:  password,
		width:        m.cfg.Width,
		height:       m.cfg.Height,
		state:        browser.SessionStateStarting,
		startedAt:    time.Now(),
		lastActivity: time.Now(),
	}

	cleanup := func() { m.teardown(created) }

	displayEnv := fmt.Sprintf(":%d", display)
	env := append(os.Environ(), "DISPLAY="+displayEnv, "DBUS_SESSION_BUS_ADDRESS=/dev/null")

	xvfb := exec.Command("Xvfb", displayEnv,
		"-screen", "0", fmt.Sprintf("%dx%dx24", m.cfg.Width, m.cfg.Height),
		"-nolisten", "tcp",
	)
	xvfb.Stdout, xvfb.Stderr = logWriter(accountID, "xvfb"), logWriter(accountID, "xvfb")
	if created.xvfb, err = startProc(xvfb); err != nil {
		return nil, fmt.Errorf("não foi possível iniciar o Xvfb: %w", err)
	}
	if err := waitForXServer(ctx, display); err != nil {
		cleanup()
		return nil, err
	}

	// O gerenciador de janelas é o que permite mover, focar e fechar os popups
	// de autenticação em duas etapas.
	wm := exec.Command("openbox", "--sm-disable")
	wm.Env = env
	wm.Stdout, wm.Stderr = logWriter(accountID, "openbox"), logWriter(accountID, "openbox")
	if created.wm, err = startProc(wm); err != nil {
		cleanup()
		return nil, fmt.Errorf("não foi possível iniciar o gerenciador de janelas: %w", err)
	}

	chrome := exec.Command(m.cfg.ChromeBinary, m.chromeArgs(profileDir, cdpPort)...)
	chrome.Env = env
	chrome.Stdout, chrome.Stderr = logWriter(accountID, "chrome"), logWriter(accountID, "chrome")
	if created.chrome, err = startProc(chrome); err != nil {
		cleanup()
		return nil, fmt.Errorf("não foi possível iniciar o navegador: %w", err)
	}
	if err := waitForPortOrExit(ctx, cdpPort, startupTimeout, created.chrome); err != nil {
		cleanup()
		return nil, err
	}

	passwordFile, err := writeVNCPasswordFile(accountID, password)
	if err != nil {
		cleanup()
		return nil, err
	}

	// -localhost prende o servidor RFB em 127.0.0.1: a porta não existe para a
	// rede do Docker, só para a ponte WebSocket deste mesmo processo.
	// "rm:" faz o x11vnc apagar o arquivo de senha logo após lê-lo.
	vnc := exec.Command("x11vnc",
		"-display", displayEnv,
		"-rfbport", fmt.Sprintf("%d", vncPort),
		"-localhost",
		"-forever",
		"-shared",
		"-noxdamage",
		"-nolookup",
		"-repeat",
		"-passwdfile", "rm:"+passwordFile,
		"-quiet",
	)
	vnc.Env = env
	vnc.Stdout, vnc.Stderr = logWriter(accountID, "x11vnc"), logWriter(accountID, "x11vnc")
	if created.vnc, err = startProc(vnc); err != nil {
		_ = os.Remove(passwordFile)
		cleanup()
		return nil, fmt.Errorf("não foi possível iniciar o servidor VNC: %w", err)
	}
	if err := waitForPortOrExit(ctx, vncPort, 15*time.Second, created.vnc); err != nil {
		_ = os.Remove(passwordFile)
		cleanup()
		return nil, err
	}
	_ = os.Remove(passwordFile)

	created.state = browser.SessionStateRunning
	created.lastActivity = time.Now()
	return created, nil
}

func (m *Manager) chromeArgs(profileDir string, cdpPort int) []string {
	args := []string{
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-dev-shm-usage",
		"--disable-gpu",
		"--disable-features=Translate,MediaRouter,AutofillServerCommunication",
		// Sem chaveiro no container: sem estas flags o Chrome trava esperando
		// o gnome-keyring. Os cookies continuam cifrados no perfil.
		"--password-store=basic",
		"--use-mock-keychain",
		"--window-position=0,0",
		fmt.Sprintf("--window-size=%d,%d", m.cfg.Width, m.cfg.Height),
		// DevTools só em 127.0.0.1 e usado exclusivamente para ler os cookies
		// da sessão. Nenhuma flag de automação é passada, para que o login do
		// Google se comporte como em um navegador comum.
		fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
		"--remote-debugging-address=127.0.0.1",
	}

	if m.cfg.DisableSandbox {
		// --test-type silencia a faixa amarela "You are using an unsupported
		// command-line flag: --no-sandbox", que o Chrome desenha no topo de
		// toda janela. Ela rouba altura da tela remota e assusta quem está
		// apenas fazendo login — o aviso é para quem opera, e já está dito no
		// README e no manifesto da stack.
		//
		// Não é flag de automação: não define navigator.webdriver nem
		// --enable-automation, e o login do Google segue normal.
		args = append(args, "--no-sandbox", "--test-type")
	}
	if m.cfg.ProxyURL != "" {
		args = append(args, "--proxy-server="+m.cfg.ProxyURL)
	}
	if m.cfg.UserAgent != "" {
		args = append(args, "--user-agent="+m.cfg.UserAgent)
	}

	return append(args, startURL)
}

// allocateDisplay procura um número de display livre, considerando tanto as
// sessões em memória quanto os locks deixados em /tmp.
func (m *Manager) allocateDisplay() (int, error) {
	m.mu.Lock()
	used := make(map[int]bool, len(m.sessions))
	for _, current := range m.sessions {
		used[current.display] = true
	}
	m.mu.Unlock()

	for display := firstDisplay; display <= lastDisplay; display++ {
		if used[display] {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X%d-lock", display)); err == nil {
			continue
		}
		return display, nil
	}
	return 0, fmt.Errorf("nenhum display gráfico disponível")
}

// waitForPortOrExit espera o processo abrir a porta, mas desiste na hora se ele
// morrer antes. Sem isso, uma falha de startup só apareceria depois do timeout
// inteiro e com uma mensagem que não ajuda a diagnosticar.
func waitForPortOrExit(ctx context.Context, port int, timeout time.Duration, p *proc) error {
	result := make(chan error, 1)
	go func() { result <- waitForPort(ctx, port, timeout) }()

	select {
	case err := <-result:
		if err != nil {
			return fmt.Errorf("o processo não ficou pronto: %w", err)
		}
		return nil
	case <-p.done:
		return fmt.Errorf("o processo encerrou durante a inicialização (código %d): %s",
			p.cmd.ProcessState.ExitCode(), startupHint(p.cmd.ProcessState.ExitCode()))
	}
}

// startupHint traduz a falha mais comum em container: o sandbox do Chrome
// precisa criar user namespaces, o que o seccomp padrão do Docker bloqueia.
func startupHint(exitCode int) string {
	if exitCode == 0 {
		return "verifique os logs do processo"
	}
	return "se for o Chrome, provavelmente o sandbox não conseguiu criar user namespaces: " +
		"use o perfil seccomp docker/chrome-seccomp.json ou defina BROWSER_DISABLE_SANDBOX=true"
}

func waitForXServer(ctx context.Context, display int) error {
	socket := fmt.Sprintf("/tmp/.X11-unix/X%d", display)
	deadline := time.Now().Add(20 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := os.Stat(socket); err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("servidor gráfico :%d não subiu", display)
}

func randomVNCPassword() (string, error) {
	limit := big.NewInt(int64(len(vncPasswordAlphabet)))
	buf := make([]byte, vncPasswordLength)

	for i := range buf {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		buf[i] = vncPasswordAlphabet[n.Int64()]
	}
	return string(buf), nil
}

// writeVNCPasswordFile grava a senha em um arquivo 0600 sob /tmp. Passar a
// senha por argumento de linha de comando a deixaria visível em `ps`.
func writeVNCPasswordFile(accountID, password string) (string, error) {
	dir := filepath.Join(os.TempDir(), "browserd")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("não foi possível preparar o diretório temporário: %w", err)
	}

	path := filepath.Join(dir, accountID+".pw")
	if err := os.WriteFile(path, []byte(password+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("não foi possível gravar a senha do VNC: %w", err)
	}
	return path, nil
}

// logNoise são mensagens que o Chrome repete às centenas em container e que não
// indicam problema: não há D-Bus, nem UPower, nem GPU aqui. Filtrá-las mantém o
// log legível para as falhas que importam.
var logNoise = []string{
	"dbus/bus.cc",
	"dbus/object_proxy.cc",
	"org.freedesktop.DBus",
	"cpufreq/scaling_",
	"XNNPACK delegate",
	"Failed to connect to the bus",
	"GetVSyncParametersIfAvailable",
	"Fontconfig error",
}

// logWriter mantém os logs dos processos filhos identificados por conta. Nada
// que passa por aqui carrega dados de sessão: o Chrome não recebe credenciais
// por linha de comando e não imprime cookies.
func logWriter(accountID, component string) *processLogger {
	return &processLogger{accountID: accountID, component: component}
}

type processLogger struct {
	accountID string
	component string

	mu sync.Mutex
	// pending guarda a linha incompleta entre duas escritas, para não registrar
	// mensagens cortadas ao meio.
	pending []byte
}

func (p *processLogger) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pending = append(p.pending, data...)

	for {
		index := bytes.IndexByte(p.pending, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimRight(string(p.pending[:index]), "\r")
		p.pending = p.pending[index+1:]
		p.emit(line)
	}

	// Uma linha gigante sem quebra não pode crescer para sempre.
	if len(p.pending) > 8<<10 {
		p.emit(strings.TrimRight(string(p.pending), "\r"))
		p.pending = nil
	}

	return len(data), nil
}

func (p *processLogger) emit(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	for _, noise := range logNoise {
		if strings.Contains(line, noise) {
			return
		}
	}
	log.Printf("browserd[%s] account_id=%s %s", p.component, p.accountID, line)
}
