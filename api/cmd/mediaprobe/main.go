// Command mediaprobe exercita o provider contra plataformas reais. É uma
// ferramenta de diagnóstico, não parte do produto: fica fora do caminho da
// aplicação e não é incluída nas imagens de produção.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/media/ytdlp"
)

func main() {
	registry := media.NewRegistry(ytdlp.New(ytdlp.Config{MetadataTimeout: 60 * time.Second}))

	ctx := context.Background()

	if len(os.Args) > 2 && os.Args[1] == "download" {
		formato := ""
		if len(os.Args) > 3 {
			formato = os.Args[3]
		}
		var cancelar time.Duration
		if len(os.Args) > 4 {
			cancelar, _ = time.ParseDuration(os.Args[4])
		}
		baixar(os.Args[2], formato, cancelar)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "health" {
		for _, saude := range registry.Health(ctx) {
			fmt.Printf("provider=%s disponível=%t versão=%s\n", saude.Provider, saude.Available, saude.Version)
			for _, ferramenta := range saude.Tools {
				fmt.Printf("   %-8s disponível=%-5t obrigatória=%-5t %s\n",
					ferramenta.Name, ferramenta.Available, ferramenta.Required, ferramenta.Version)
			}
		}
		return
	}

	for _, bruta := range os.Args[1:] {
		plataforma, escolhido, normalizada, err := registry.Resolve(bruta)
		if err != nil {
			fmt.Printf("%-12s RECUSADA        %s\n", "-", media.UserMessage(err))
			continue
		}

		inicio := time.Now()
		metadata, _, err := registry.Metadata(ctx, normalizada)
		decorrido := time.Since(inicio).Round(100 * time.Millisecond)

		if err != nil {
			fmt.Printf("%-12s FALHOU (%s)  %s\n", plataforma, decorrido, media.UserMessage(err))
			var domainErr *media.Error
			if ok := asDomain(err, &domainErr); ok && domainErr.Detail != "" {
				fmt.Printf("             detalhe: %s\n", domainErr.Detail)
			}
			continue
		}

		fmt.Printf("%-12s OK (%s) provider=%s\n", plataforma, decorrido, escolhido.Name())
		fmt.Printf("             título   : %s\n", corta(metadata.Title, 60))
		fmt.Printf("             autor    : %s | duração: %ds\n", corta(metadata.Uploader, 30), metadata.Duration)
		fmt.Printf("             thumbnail: %t | formatos: %d\n", metadata.Thumbnail != "", len(metadata.Formats))
		for i, formato := range metadata.Formats {
			if i >= 4 {
				fmt.Printf("             ... e mais %d\n", len(metadata.Formats)-4)
				break
			}
			fmt.Printf("               - %-16s id=%-8s %s %dMB\n",
				formato.Label, formato.ID, formato.Ext, formato.SizeBytes/1024/1024)
		}
	}
}

func asDomain(err error, alvo **media.Error) bool {
	if convertido, ok := err.(*media.Error); ok {
		*alvo = convertido
		return true
	}
	return false
}

func corta(texto string, limite int) string {
	if len(texto) <= limite {
		return texto
	}
	return texto[:limite] + "…"
}
