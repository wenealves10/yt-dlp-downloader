package helpers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
)

func DownloadImage(ctx context.Context, imageURL, outputPath string) error {
	if _, err := os.Stat(outputPath); err == nil {
		return nil
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretórios: %w", err)
	}

	var clientHttp *http.Client

	if configs.LoadedConfig.ProxyEnabled {
		proxyUrl, err := url.Parse(configs.LoadedConfig.ProxyURL)
		if err != nil {
			log.Fatalln(err)
		}
		clientHttp = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)},
		}
	} else {
		clientHttp = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}

	resp, err := clientHttp.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao baixar imagem: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resposta HTTP inválida: %s", resp.Status)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("erro ao criar arquivo: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao salvar imagem: %w", err)
	}

	return nil
}
