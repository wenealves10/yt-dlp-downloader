package jobs

import (
	"log"

	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

// abortadoPeloUsuario decide se um upload ainda deve acontecer.
//
// O cancelamento pode chegar depois de o download terminar e antes de o upload
// começar — a tarefa fica na fila, e essa janela é justamente a mais provável
// num arquivo grande. Sem esta checagem o arquivo ia parar no storage mesmo
// tendo sido cancelado, e ficava lá ocupando espaço e cota até expirar.
//
// Os caminhos recebidos são apagados aqui: quem cancela não quer sobra nem no
// disco do worker nem no bucket.
func abortadoPeloUsuario(download db.Download, caminhos ...string) bool {
	if download.Status != db.CoreDownloadStatusCANCELED {
		return false
	}

	log.Printf("jobs: upload descartado, download cancelado id=%s", download.ID)
	for _, caminho := range caminhos {
		if caminho == "" {
			continue
		}
		if err := utils.RemoveFile(caminho); err != nil {
			log.Printf("jobs: falha ao remover arquivo de download cancelado id=%s: %v",
				download.ID, err)
		}
	}
	return true
}
