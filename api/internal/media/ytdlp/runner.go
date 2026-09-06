// Package ytdlp implementa media.Provider sobre o binário yt-dlp. É o ÚNICO
// lugar do sistema que sabe que o yt-dlp existe: nome do binário, argumentos,
// formato do progresso e texto dos erros ficam todos contidos aqui.
package ytdlp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// maxStderrBytes limita o que guardamos da saída de erro. O yt-dlp pode
// despejar centenas de linhas; queremos o suficiente para diagnosticar sem
// carregar isso para dentro de uma coluna de banco ou de um log.
const maxStderrBytes = 8 << 10

// runner executa o binário. Nada aqui monta comando por concatenação de
// string: os argumentos vão estruturados para o exec, que não passa por shell.
// É o que impede que uma URL com `;` ou `$(...)` vire comando.
type runner struct {
	binary  string
	proxy   string
	timeout time.Duration
}

// coletorStderr guarda as últimas linhas de erro sem deixar a memória crescer.
type coletorStderr struct {
	mu    sync.Mutex
	buf   strings.Builder
	cheio bool
}

func (c *coletorStderr) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cheio {
		restante := maxStderrBytes - c.buf.Len()
		if restante > 0 {
			if len(p) <= restante {
				c.buf.Write(p)
			} else {
				c.buf.Write(p[:restante])
				c.cheio = true
			}
		} else {
			c.cheio = true
		}
	}
	return len(p), nil
}

func (c *coletorStderr) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// executar roda o yt-dlp com os argumentos dados e entrega cada linha do stdout
// a onLine. Devolve o stderr coletado junto do erro, para diagnóstico.
func (r *runner) executar(
	ctx context.Context,
	args []string,
	onLine func(string),
) (stderr string, err error) {
	if r.timeout > 0 {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithTimeout(ctx, r.timeout)
		defer cancelar()
	}

	cmd := exec.CommandContext(ctx, r.binary, args...)

	// Setpgid coloca o yt-dlp e os filhos dele (ffmpeg) no mesmo grupo. Sem
	// isso, cancelar mataria só o yt-dlp e o ffmpeg continuaria escrevendo em
	// disco — o processo órfão clássico.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// O padrão do exec mata só o processo direto; aqui o grupo inteiro cai.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if pgid, erroGrupo := syscall.Getpgid(cmd.Process.Pid); erroGrupo == nil {
			return syscall.Kill(-pgid, syscall.SIGKILL)
		}
		return cmd.Process.Kill()
	}

	coletor := &coletorStderr{}
	cmd.Stderr = coletor

	saida, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	// A leitura acontece em paralelo ao processo; ler depois do Wait perderia
	// o progresso e poderia encher o buffer do pipe e travar o filho.
	leitura := make(chan struct{})
	go func() {
		defer close(leitura)
		scanner := bufio.NewScanner(saida)
		// Linhas de progresso são curtas, mas uma linha de JSON de metadados
		// passa fácil dos 64 KB do buffer padrão.
		scanner.Buffer(make([]byte, 0, 64<<10), 8<<20)
		for scanner.Scan() {
			if onLine != nil {
				onLine(scanner.Text())
			}
		}
		if erroScanner := scanner.Err(); erroScanner != nil && !errors.Is(erroScanner, io.ErrClosedPipe) {
			// Não é fatal: o Wait abaixo dirá se o processo falhou.
			coletor.Write([]byte("\n[stdout] " + erroScanner.Error()))
		}
	}()

	<-leitura
	erroEspera := cmd.Wait()

	if ctxErr := ctx.Err(); ctxErr != nil {
		return coletor.String(), ctxErr
	}
	return coletor.String(), erroEspera
}

// versao devolve a versão do binário, ou erro quando ele não está utilizável.
func (r *runner) versao(ctx context.Context) (string, error) {
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()

	saida, err := exec.CommandContext(ctx, r.binary, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(saida)), nil
}
