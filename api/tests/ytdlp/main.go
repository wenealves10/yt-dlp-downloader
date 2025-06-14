package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", "output.mp3",
		"--progress", // ativa o progresso
		"--newline",  // permite capturar o progresso linha a linha
		"https://www.youtube.com/watch?v=HWjoQ92VKEs",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	// regex para capturar porcentagem como: " 42.5%"
	progressRegex := regexp.MustCompile(`(?m)(\d{1,3}\.\d)%`)

	// Lê saída STDOUT
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()

			if strings.Contains(line, "[download]") {
				matches := progressRegex.FindStringSubmatch(line)
				if len(matches) > 1 {
					fmt.Printf("📊 Progresso: %s%%\n", matches[1])
				}
			}
		}
	}()

	logFileName := fmt.Sprintf("log_%s_%s.txt", uuid.New().String(), time.Now().Format("2006-01-02_15-04-05"))

	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Fatalf("Erro ao criar arquivo de log: %v", err)
	}
	defer logFile.Close()

	writer := bufio.NewWriter(logFile)
	defer writer.Flush()

	// Lê saída STDERR
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Escreve no arquivo de log
			if _, err := writer.WriteString("[STDERR] " + line + "\n"); err != nil {
				log.Printf("Erro ao escrever no arquivo de log: %v", err)
			}
		}
	}()

	// Espera o processo terminar
	if err := cmd.Wait(); err != nil {
		fmt.Printf("Erro ao executar: %v\n", err)
		fmt.Fprintf(writer, "Erro final: %v\n", err)
	}
}
