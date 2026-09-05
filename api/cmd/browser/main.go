// Command browser executa o serviço de navegador remoto (advideo-browser).
//
// É o único processo do sistema que abre o Chrome. Ele não publica portas para
// fora da rede interna do Docker, exige o segredo compartilhado em toda rota e
// mantém os perfis persistentes em um volume dedicado.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/browserd"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
)

func main() {
	cg, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	if cg.BrowserServiceToken == "" {
		log.Fatal("BROWSER_SERVICE_TOKEN é obrigatório: o serviço não sobe sem o segredo compartilhado")
	}

	profilesDir := cg.BrowserProfilesDir
	if profilesDir == "" {
		profilesDir = "/data/profiles"
	}

	listenAddress := cg.BrowserListenAddress
	if listenAddress == "" {
		listenAddress = "0.0.0.0:9223"
	}

	width, height := parseScreenSize(cg.BrowserScreenSize)

	// O navegador remoto sai pelo mesmo proxy dos downloads quando ele está
	// habilitado, para que a sessão seja criada e usada pelo mesmo caminho de
	// rede.
	proxyURL := ""
	if cg.ProxyEnabled {
		proxyURL = cg.ProxyURL
	}

	manager, err := browserd.NewManager(browserd.Config{
		ProfilesDir:    profilesDir,
		ChromeBinary:   cg.BrowserBinary,
		IdleTimeout:    cg.BrowserIdleTimeout,
		Width:          width,
		Height:         height,
		MaxSessions:    cg.BrowserMaxSessions,
		DisableSandbox: cg.BrowserDisableSandbox,
		ProxyURL:       proxyURL,
		UserAgent:      cg.YoutubeDLUserAgent,
	})
	if err != nil {
		log.Fatalf("cannot create browser manager: %v", err)
	}

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           browserd.NewServer(manager, cg.BrowserServiceToken).Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("🖥️  advideo-browser ouvindo em %s (perfis em %s)", listenAddress, profilesDir)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("cannot start browser service: %v", err)
		}
	}()

	<-shutdown
	log.Println("advideo-browser: encerrando sessões abertas")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("advideo-browser: erro no shutdown do HTTP: %v", err)
	}
	manager.Close()
}

// parseScreenSize aceita "1440x900" e cai para um padrão seguro em qualquer
// valor inválido.
func parseScreenSize(value string) (int, int) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(value)), "x", 2)
	if len(parts) != 2 {
		return 1440, 900
	}

	width, errWidth := strconv.Atoi(parts[0])
	height, errHeight := strconv.Atoi(parts[1])
	if errWidth != nil || errHeight != nil || width < 800 || height < 600 || width > 2560 || height > 1600 {
		return 1440, 900
	}
	return width, height
}
