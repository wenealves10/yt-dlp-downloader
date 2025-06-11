# 📽️ yt-dlp-downloader

**yt-dlp-downloader** é um MVP (Minimum Viable Product) construído com **Go** para realizar **downloads de vídeos a partir de URLs (como YouTube)** de forma assíncrona, escalável e com notificações em tempo real para o usuário.

---

## 🚀 Tecnologias Utilizadas

| Tecnologia      | Função                                                    |
|----------------|-----------------------------------------------------------|
| **Go (Golang)** | Backend performático e conciso                           |
| **Fiber**       | Framework HTTP leve e rápido para criação de APIs REST   |
| **Asynq**       | Gerenciador de tarefas assíncronas via Redis             |
| **yt-dlp**      | Ferramenta de linha de comando para baixar vídeos        |
| **Redis**       | Fila de tarefas e Pub/Sub para comunicação de eventos    |
| **SSE (Server-Sent Events)** | Comunicação em tempo real do backend para o frontend |

---

## 🧱 Funcionalidades

- 🔗 Envio de URL para download (`POST /downloads`)
- ⏳ Processamento assíncrono com fila (Redis + Asynq)
- 📥 Execução de download com yt-dlp
- 🔁 Envio de status em tempo real via SSE (`GET /downloads/:id/stream`)
- 📊 Histórico e status de tarefas (em breve com banco de dados)
- ⚙️ Pronto para escalar com múltiplos workers

---

## 📂 Estrutura de Pastas

```bash
yt-dlp-downloader/
├── cmd/
│   ├── server/     # Servidor HTTP com Fiber
│   └── worker/     # Worker Asynq que processa os downloads
├── internal/
│   ├── api/        # Handlers e rotas
│   ├── jobs/       # Definições de tarefas e payloads
│   ├── services/   # Lógica de negócio (yt-dlp, SSE, etc)
│   ├── sse/        # Gerenciamento de conexões SSE
│   └── models/     # Estruturas e modelos de dados
├── docker-compose.yml  # Redis e serviços auxiliares
├── go.mod
└── README.md
```

---

## 🧪 Como Rodar Localmente

1. Instale o Redis (ou use `docker-compose`):
   ```bash
   docker-compose up -d
   ```

2. Inicie o servidor HTTP:
   ```bash
   go run cmd/server/main.go
   ```

3. Inicie o worker:
   ```bash
   go run cmd/worker/main.go
   ```

4. Faça uma requisição:
   ```bash
   curl -X POST http://localhost:3000/downloads \
     -H "Content-Type: application/json" \
     -d '{ "url": "https://youtube.com/watch?v=dQw4w9WgXcQ" }'
   ```

---

## 📌 Próximas Melhorias

- [ ] Armazenamento persistente no PostgreSQL
- [ ] Upload automático para armazenamento em nuvem
- [ ] Dashboard com histórico e progresso em tempo real
- [ ] Autenticação e controle de acesso

---

Desenvolvido com 💻 por **Alves** ✨
