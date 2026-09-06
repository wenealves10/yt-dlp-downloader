package jobs

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// JobMediaHealth publica no Redis o diagnóstico do provider deste processo.
//
// O painel administrativo roda na API, mas o download acontece aqui: sem esta
// publicação, a tela inspecionaria o container errado e reportaria como falha
// uma dependência que o processo dela nunca usa.
type JobMediaHealth struct {
	redis    *redis.Client
	registry *media.Registry
	role     media.Role
}

func NewJobMediaHealth(redisClient *redis.Client, registry *media.Registry, role media.Role) *JobMediaHealth {
	return &JobMediaHealth{redis: redisClient, registry: registry, role: role}
}

func (p *JobMediaHealth) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	p.Publicar(ctx)
	return nil
}

// Publicar coleta e grava o relatório. Uma falha aqui não derruba nada: o
// painel apenas mostra o relatório como indisponível.
func (p *JobMediaHealth) Publicar(ctx context.Context) {
	if p.redis == nil || p.registry == nil {
		return
	}

	consultaCtx, cancelar := context.WithTimeout(ctx, 30*time.Second)
	defer cancelar()

	relatorio := media.HealthReport{
		Role:       p.role,
		Providers:  p.registry.Health(consultaCtx),
		ReportedAt: time.Now().UTC(),
	}

	bruto, err := json.Marshal(relatorio)
	if err != nil {
		log.Printf("jobs: não foi possível serializar a saúde do provider: %v", err)
		return
	}

	if err := p.redis.Set(ctx, media.HealthKey(p.role), bruto, media.HealthTTL).Err(); err != nil {
		log.Printf("jobs: não foi possível publicar a saúde do provider: %v", err)
		return
	}

	for _, saude := range relatorio.Providers {
		if !saude.Available {
			log.Printf("jobs: provider %s indisponível neste worker: %s", saude.Provider, saude.Detail)
		}
	}
}
