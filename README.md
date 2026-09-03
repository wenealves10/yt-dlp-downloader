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

## 🐳 Ambiente de desenvolvimento com Docker

O ambiente local é isolado em `compose.dev.yaml`: cria PostgreSQL, Redis, API,
worker e o app Vite. Ele usa volumes e portas de desenvolvimento próprios. O
armazenamento de arquivos continua usando o Cloudflare R2 já definido em
`api/.env` e `app/.env.local`; nenhum MinIO é criado.

`api/.env` é a fonte única das variáveis de Redis e PostgreSQL: o `make dev`
passa esse arquivo ao Compose e ele também é carregado pela API e pelo worker.
Para o Compose, mantenha os nomes e portas internos (`REDIS_HOST=redis`,
`REDIS_PORT=6379`, `DB_HOST=postgres` e `DB_PORT=5432`), além do `DB_SOURCE`
com o host `postgres`. As portas `5433` e `6380` abaixo são apenas os acessos
equivalentes a partir da máquina host.

Se o arquivo ainda não existir, comece pelo modelo e preencha as variáveis do
Cloudflare R2 que forem necessárias:

```bash
cp api/.env.example api/.env
```

```bash
make dev
```

O comando pode ser executado novamente. As migrations pendentes são aplicadas
automaticamente e as já aplicadas são ignoradas. Os endereços padrão são:

- App: `http://localhost:5173`
- API: `http://localhost:8081`
- PostgreSQL: `localhost:5433`
- Redis: `localhost:6380`

As portas podem ser alteradas sem editar arquivos, por exemplo:

```bash
DEV_APP_PORT=5174 DEV_API_PORT=8082 make dev
```

Comandos úteis:

```bash
make help
make dev-logs
make dev-migrate
make dev-down
```

---

## 📌 Próximas Melhorias

- [ ] Armazenamento persistente no PostgreSQL
- [ ] Upload automático para armazenamento em nuvem
- [ ] Dashboard com histórico e progresso em tempo real
- [ ] Autenticação e controle de acesso

---

Desenvolvido com 💻 por **Alves** ✨
