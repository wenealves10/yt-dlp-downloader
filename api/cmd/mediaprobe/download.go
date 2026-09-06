package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/media/ytdlp"
)

// baixar exercita o caminho completo de download, imprimindo o progresso como
// o worker o receberá.
func baixar(url, formatoID string, cancelarApos time.Duration) {
	provider := ytdlp.New(ytdlp.Config{DownloadTimeout: 10 * time.Minute})

	dir, err := os.MkdirTemp("", "probe-")
	if err != nil {
		fmt.Println("erro ao criar diretório:", err)
		return
	}
	defer os.RemoveAll(dir)

	ctx := context.Background()
	if cancelarApos > 0 {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithCancel(ctx)
		go func() {
			time.Sleep(cancelarApos)
			fmt.Println("   >> cancelando...")
			cancelar()
		}()
	}

	inicio := time.Now()
	var atualizacoes int

	resultado, err := provider.Download(ctx, media.Request{
		URL:       url,
		FormatID:  formatoID,
		Kind:      media.KindVideo,
		OutputDir: dir,
		Filename:  "probe_arquivo",
	}, func(p media.Progress) {
		atualizacoes++
		if p.Postprocess {
			fmt.Printf("   pós-processando...\n")
			return
		}
		fmt.Printf("   %5.1f%%  %6.1f MB / %6.1f MB  %5.1f MB/s  ETA %ds\n",
			p.Percent,
			float64(p.DownloadedBytes)/1024/1024,
			float64(p.TotalBytes)/1024/1024,
			float64(p.SpeedBPS)/1024/1024,
			p.ETASeconds)
	})

	if err != nil {
		fmt.Printf("   resultado: %s (%d atualizações de progresso)\n",
			media.UserMessage(err), atualizacoes)
		// O diretório temporário precisa ficar limpo mesmo em falha.
		sobras, _ := os.ReadDir(dir)
		fmt.Printf("   arquivos deixados no temporário: %d\n", len(sobras))
		return
	}

	fmt.Printf("   concluído em %s: %s (%.1f MB, .%s) — %d atualizações\n",
		time.Since(inicio).Round(time.Second), resultado.FilePath,
		float64(resultado.SizeBytes)/1024/1024, resultado.Ext, atualizacoes)
}
