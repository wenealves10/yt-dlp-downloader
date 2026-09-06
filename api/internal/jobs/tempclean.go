package jobs

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

// idadeMinimaParaLimpar protege os downloads em andamento: um diretório recém
// criado pode ser de um job vivo neste mesmo processo.
const idadeMinimaParaLimpar = 6 * time.Hour

// LimparTemporariosOrfaos remove diretórios de trabalho deixados para trás. O
// job limpa o próprio diretório ao terminar, mas um container derrubado no meio
// de um download morre sem rodar essa limpeza — e um vídeo grande deixa alguns
// gigabytes ocupados para sempre.
//
// Roda na subida do worker e periodicamente: as duas ocasiões em que sabemos
// que nada mais aponta para o que ficou.
func LimparTemporariosOrfaos(workDir string) {
	if workDir == "" {
		workDir = "./uploads/tmp"
	}

	entradas, err := os.ReadDir(workDir)
	if err != nil {
		// Diretório ainda não existe na primeira subida: não é problema.
		if !os.IsNotExist(err) {
			log.Printf("jobs: não foi possível inspecionar %s: %v", workDir, err)
		}
		return
	}

	agora := time.Now()
	var removidos int
	var bytesLiberados int64

	for _, entrada := range entradas {
		// Só os diretórios que este pacote cria; nada mais é tocado.
		if !entrada.IsDir() || !strings.HasPrefix(entrada.Name(), "dl-") {
			continue
		}

		info, err := entrada.Info()
		if err != nil || agora.Sub(info.ModTime()) < idadeMinimaParaLimpar {
			continue
		}

		caminho := filepath.Join(workDir, entrada.Name())
		tamanho := tamanhoDe(caminho)
		if err := os.RemoveAll(caminho); err != nil {
			log.Printf("jobs: falha ao remover temporário %s: %v", entrada.Name(), err)
			continue
		}
		removidos++
		bytesLiberados += tamanho
	}

	if removidos > 0 {
		log.Printf("jobs: %d diretório(s) temporário(s) órfão(s) removido(s), %.1f MB liberados",
			removidos, float64(bytesLiberados)/1024/1024)
	}
}

func tamanhoDe(caminho string) int64 {
	var total int64
	_ = filepath.WalkDir(caminho, func(_ string, entrada os.DirEntry, err error) error {
		if err != nil || entrada.IsDir() {
			return nil
		}
		if info, err := entrada.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// JobTempCleanup expõe a limpeza como tarefa agendada.
type JobTempCleanup struct {
	workDir string
}

func NewJobTempCleanup(workDir string) *JobTempCleanup {
	return &JobTempCleanup{workDir: workDir}
}

func (p *JobTempCleanup) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	LimparTemporariosOrfaos(p.workDir)
	return nil
}
