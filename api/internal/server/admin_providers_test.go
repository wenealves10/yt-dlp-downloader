package server

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

func etapaBase() etapaSaude {
	return etapaSaude{
		Role:      media.RoleDownloader,
		Label:     media.RoleDownloader.Label(),
		Source:    "worker",
		Providers: []media.Health{},
	}
}

// Worker sem relatório não é worker quebrado: pode estar parado, subindo ou
// numa versão anterior à publicação. Confundir os dois casos mandaria alguém
// procurar defeito no yt-dlp quando o problema é o container.
func TestEtapaDownloadSemRelatorio(t *testing.T) {
	etapa := interpretarRelatorioWorker(etapaBase(), "", redis.Nil)

	if !etapa.Stale {
		t.Error("a ausência do relatório deveria marcar a etapa como sem dado")
	}
	if etapa.Available {
		t.Error("sem relatório não se pode afirmar que está saudável")
	}
	if etapa.Detail == "" {
		t.Error("a tela precisa de um motivo para mostrar")
	}
}

func TestEtapaDownloadRelatorioIlegivel(t *testing.T) {
	etapa := interpretarRelatorioWorker(etapaBase(), "{isto não é json", nil)

	if !etapa.Stale {
		t.Error("um relatório ilegível deveria ser tratado como ausente")
	}
	if len(etapa.Providers) != 0 {
		t.Error("nada deveria ser exibido a partir de um relatório inválido")
	}
}

// O caso que motivou tudo isto: o worker tem ffmpeg e está saudável, e a tela
// precisa dizer isso mesmo rodando na API, que não tem ffmpeg nenhum.
func TestEtapaDownloadRelatorioSaudavel(t *testing.T) {
	reportado := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	bruto, err := json.Marshal(media.HealthReport{
		Role:       media.RoleDownloader,
		ReportedAt: reportado,
		Providers: []media.Health{{
			Provider:  "yt-dlp",
			Role:      media.RoleDownloader,
			Available: true,
			Version:   "2026.8.19",
			Tools: []media.Tool{
				{Name: "yt-dlp", Available: true, Required: true},
				{Name: "ffmpeg", Available: true, Required: true},
			},
		}},
	})
	if err != nil {
		t.Fatalf("não foi possível montar o relatório: %v", err)
	}

	etapa := interpretarRelatorioWorker(etapaBase(), string(bruto), nil)

	if etapa.Stale {
		t.Error("um relatório fresco não deveria ser marcado como ausente")
	}
	if !etapa.Available {
		t.Error("worker com todas as dependências deveria aparecer como saudável")
	}
	if etapa.ReportedAt == nil || !etapa.ReportedAt.Equal(reportado) {
		t.Errorf("o horário do relatório deveria ser preservado, obtido %v", etapa.ReportedAt)
	}
	if len(etapa.Providers) != 1 || etapa.Providers[0].Provider != "yt-dlp" {
		t.Errorf("o provider do worker deveria ser repassado, obtido %+v", etapa.Providers)
	}
}

// Um provider indisponível no worker é falha de verdade, e não pode ser
// confundido com relatório ausente — é a diferença entre "reinicie o worker" e
// "a imagem está sem uma dependência".
func TestEtapaDownloadProviderIndisponivel(t *testing.T) {
	bruto, err := json.Marshal(media.HealthReport{
		Role:       media.RoleDownloader,
		ReportedAt: time.Now().UTC(),
		Providers: []media.Health{{
			Provider:  "yt-dlp",
			Available: false,
			Detail:    "ffmpeg indisponível: faixas separadas não podem ser unidas",
		}},
	})
	if err != nil {
		t.Fatalf("não foi possível montar o relatório: %v", err)
	}

	etapa := interpretarRelatorioWorker(etapaBase(), string(bruto), nil)

	if etapa.Stale {
		t.Error("há relatório: a etapa não está sem dado, está com defeito")
	}
	if etapa.Available {
		t.Error("provider indisponível deveria derrubar a etapa")
	}
}

func TestEtapaDownloadErroDeLeitura(t *testing.T) {
	etapa := interpretarRelatorioWorker(etapaBase(), "", errors.New("conexão recusada"))

	if !etapa.Stale || etapa.Available {
		t.Error("falha ao ler o Redis deveria virar etapa sem dado")
	}
}

// Lista vazia não é sinal de saúde: nenhum provider registrado significa que
// nada seria capaz de baixar.
func TestTodosDisponiveisRejeitaListaVazia(t *testing.T) {
	if todosDisponiveis(nil) {
		t.Error("uma lista vazia não deveria contar como saudável")
	}
}
