package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// etapaSaude é o diagnóstico de um dos dois processos que executam o provider.
//
// A separação existe porque as dependências não são as mesmas: a API só resolve
// metadados e nunca chama ffmpeg; quem baixa e junta as faixas é o worker. Um
// diagnóstico único cobraria da API uma dependência que ela não usa e reportaria
// como falha algo que nunca quebrou.
type etapaSaude struct {
	Role       media.Role     `json:"role"`
	Label      string         `json:"label"`
	Source     string         `json:"source"`
	Providers  []media.Health `json:"providers"`
	Available  bool           `json:"available"`
	Detail     string         `json:"detail,omitempty"`
	ReportedAt *time.Time     `json:"reported_at,omitempty"`
	Stale      bool           `json:"stale"`
}

// listProviders diagnostica o mecanismo de download. É o que responde
// "por que nenhum download está funcionando?" sem entrar no container.
func (s *Server) listProviders(ctx *gin.Context) {
	requestCtx, cancelar := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
	defer cancelar()

	etapas := []etapaSaude{
		s.etapaResolucao(requestCtx),
		s.etapaDownload(requestCtx),
	}

	saudavel := true
	for _, etapa := range etapas {
		if !etapa.Available {
			saudavel = false
		}
	}

	plataformas := make([]gin.H, 0, len(media.KnownPlatforms()))
	for _, plataforma := range media.KnownPlatforms() {
		plataformas = append(plataformas, gin.H{
			"id":    string(plataforma),
			"label": plataforma.Label(),
			// Reconhecida não é o mesmo que baixável: o Kwai é identificado
			// pela URL e recusado na hora, porque não existe extractor para
			// ele. Listar os dois casos igual faria a tela prometer o que não
			// entrega.
			"supported": plataforma.TemSuporte(),
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"stages":  etapas,
		"healthy": saudavel,
		// A lista do que o yt-dlp entende passa de mil sites e muda a cada
		// versão dele; estas são as plataformas com rótulo próprio na tela.
		"platforms": plataformas,
	})
}

// etapaResolucao roda aqui mesmo: é este processo que responde ao /resolve.
func (s *Server) etapaResolucao(ctx context.Context) etapaSaude {
	etapa := etapaSaude{
		Role:   media.RoleResolver,
		Label:  media.RoleResolver.Label(),
		Source: "api",
	}

	if s.mediaRegistry == nil {
		etapa.Providers = []media.Health{}
		etapa.Detail = "nenhum provider registrado neste ambiente"
		return etapa
	}

	agora := time.Now().UTC()
	etapa.Providers = s.mediaRegistry.Health(ctx)
	etapa.ReportedAt = &agora
	etapa.Available = todosDisponiveis(etapa.Providers)
	return etapa
}

// etapaDownload lê o relatório que o worker publica. Executar o diagnóstico
// aqui não diria nada sobre ele: são containers e imagens diferentes.
func (s *Server) etapaDownload(ctx context.Context) etapaSaude {
	etapa := etapaSaude{
		Role:      media.RoleDownloader,
		Label:     media.RoleDownloader.Label(),
		Source:    "worker",
		Providers: []media.Health{},
	}

	if s.redis == nil {
		etapa.Stale = true
		etapa.Detail = "sem Redis para ler o relatório do worker"
		return etapa
	}

	bruto, err := s.redis.Get(ctx, media.HealthKey(media.RoleDownloader)).Result()
	return interpretarRelatorioWorker(etapa, bruto, err)
}

// interpretarRelatorioWorker é separado da leitura para poder ser testado sem
// um Redis de verdade: é aqui que mora a decisão entre "worker com problema" e
// "worker sem relatório", que são coisas bem diferentes para quem lê a tela.
func interpretarRelatorioWorker(etapa etapaSaude, bruto string, err error) etapaSaude {
	switch {
	case errors.Is(err, redis.Nil):
		etapa.Stale = true
		// O relatório expira em 10 minutos: ausência significa worker parado ou
		// numa versão anterior a esta, não provider quebrado.
		etapa.Detail = "o worker não publicou diagnóstico nos últimos minutos"
		return etapa
	case err != nil:
		etapa.Stale = true
		etapa.Detail = "não foi possível ler o relatório do worker"
		return etapa
	}

	var relatorio media.HealthReport
	if err := json.Unmarshal([]byte(bruto), &relatorio); err != nil {
		etapa.Stale = true
		etapa.Detail = "o relatório publicado pelo worker está ilegível"
		return etapa
	}

	etapa.Providers = relatorio.Providers
	if etapa.Providers == nil {
		etapa.Providers = []media.Health{}
	}
	if !relatorio.ReportedAt.IsZero() {
		reportado := relatorio.ReportedAt.UTC()
		etapa.ReportedAt = &reportado
	}
	etapa.Available = todosDisponiveis(etapa.Providers)
	return etapa
}

func todosDisponiveis(saude []media.Health) bool {
	if len(saude) == 0 {
		return false
	}
	for _, item := range saude {
		if !item.Available {
			return false
		}
	}
	return true
}
