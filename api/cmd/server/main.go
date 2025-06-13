package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/pkg/sse"
)

var manager = sse.NewSSEManager()

func main() {
	http.HandleFunc("/events", streamHandler)
	http.HandleFunc("/send", sendHandler)

	log.Println("Servidor iniciado em http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "falta o id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := manager.Subscribe(id)
	defer manager.Unsubscribe(id, sub)

	notify := w.(http.Flusher)

	for {
		select {
		case msg := <-sub:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			notify.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "falta o id", http.StatusBadRequest)
		return
	}

	msg := fmt.Sprintf("Hora atual: %s", time.Now().Format(time.RFC3339))
	manager.Publish(id, msg)

	fmt.Fprintf(w, "Mensagem enviada para id %s\n", id)
}
