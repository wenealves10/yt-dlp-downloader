package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// cacheMetadataTTL cobre com folga o tempo entre a tela mostrar as opções e o
// usuário clicar. Curto de propósito: metadados envelhecem, e um conteúdo
// removido não deve continuar aceitando pedido por horas.
const cacheMetadataTTL = 15 * time.Minute

// O identificador de formato NÃO é estável entre resoluções: para o mesmo
// vídeo, o yt-dlp devolve conjuntos diferentes em chamadas distintas — o "18"
// que a tela ofereceu pode não existir segundos depois. Guardar a resolução e
// reusá-la na criação é o que faz a escolha do usuário valer.
//
// De quebra, evita uma segunda ida à plataforma: a criação fica instantânea em
// vez de esperar mais alguns segundos.
func chaveMetadata(urlNormalizada string) string {
	soma := sha256.Sum256([]byte(urlNormalizada))
	return "media:meta:" + hex.EncodeToString(soma[:16])
}

func (s *Server) guardarMetadata(ctx context.Context, urlNormalizada string, metadata *media.Metadata) {
	if s.redis == nil || metadata == nil {
		return
	}

	bruto, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	if err := s.redis.Set(ctx, chaveMetadata(urlNormalizada), bruto, cacheMetadataTTL).Err(); err != nil {
		// Sem cache o fluxo ainda funciona: a criação resolve de novo.
		log.Printf("media: não foi possível guardar os metadados em cache: %v", err)
	}
}

func (s *Server) metadataEmCache(ctx context.Context, urlNormalizada string) *media.Metadata {
	if s.redis == nil {
		return nil
	}

	bruto, err := s.redis.Get(ctx, chaveMetadata(urlNormalizada)).Bytes()
	if err != nil {
		return nil
	}

	var metadata media.Metadata
	if err := json.Unmarshal(bruto, &metadata); err != nil {
		return nil
	}
	return &metadata
}
