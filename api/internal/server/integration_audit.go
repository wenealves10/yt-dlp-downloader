package server

import (
	"context"
	"log"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// Tamanho da fila de auditoria. Cada linha é pequena; o buffer existe para
// absorver rajada, não para guardar histórico.
const filaAuditoria = 4096

// auditor grava a auditoria de requisições FORA do caminho da resposta.
//
// A alternativa seria gravar no próprio middleware, depois de a resposta sair.
// O cliente não esperaria por isso, mas a goroutine da conexão sim — e sob
// carga isso significa dobrar as idas ao Postgres por requisição, competindo
// com as consultas que de fato atendem gente. Uma fila em memória com
// escritores dedicados troca esse custo por um atraso de alguns milissegundos
// no registro.
//
// O contrato aceito aqui é explícito: sob pressão, PERDE-SE REGISTRO, nunca
// disponibilidade. Um lote de linhas de auditoria descartadas atrapalha uma
// investigação; recusar requisições porque a tabela de log não vaza é pior.
type auditor struct {
	store db.Store
	fila  chan db.CreateIntegrationRequestParams

	// Controla a frequência do aviso de descarte: fila cheia produziria uma
	// linha de log por requisição, o que afogaria justamente o log onde se
	// procuraria a causa.
	ultimoAviso time.Time
}

func novoAuditor(store db.Store) *auditor {
	return &auditor{
		store: store,
		fila:  make(chan db.CreateIntegrationRequestParams, filaAuditoria),
	}
}

// iniciar sobe os escritores. Dois bastam: a escrita é um INSERT sem retorno, e
// mais escritores só aumentariam a disputa pelo pool de conexões.
func (a *auditor) iniciar(ctx context.Context) {
	if a == nil {
		return
	}
	for i := 0; i < 2; i++ {
		go a.escrever(ctx)
	}
}

func (a *auditor) escrever(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case registro := <-a.fila:
			// Contexto próprio, desligado do da requisição (que já terminou) e
			// com teto: um INSERT preso não pode travar o escritor para sempre.
			gravacaoCtx, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if err := a.store.CreateIntegrationRequest(gravacaoCtx, registro); err != nil {
				log.Printf("integracoes: falha ao gravar auditoria integracao=%s: %v",
					registro.IntegrationID, err)
			}
			cancelar()
		}
	}
}

// registrar enfileira sem bloquear.
func (a *auditor) registrar(registro db.CreateIntegrationRequestParams) {
	if a == nil {
		return
	}
	select {
	case a.fila <- registro:
	default:
		if time.Since(a.ultimoAviso) > time.Second {
			a.ultimoAviso = time.Now()
			log.Printf("integracoes: fila de auditoria cheia; registro descartado integracao=%s",
				registro.IntegrationID)
		}
	}
}
