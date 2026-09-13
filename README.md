# 📽️ yt-dlp-downloader

**yt-dlp-downloader** é um MVP (Minimum Viable Product) construído com **Go** para realizar **downloads de vídeos a partir de URLs (como YouTube)** de forma assíncrona, escalável e com notificações em tempo real para o usuário.

---

## 🚀 Tecnologias Utilizadas

| Tecnologia      | Função                                                    |
|----------------|-----------------------------------------------------------|
| **Go (Golang)** | Backend performático e conciso                           |
| **Gin**         | Framework HTTP usado pela API REST                       |
| **Asynq**       | Gerenciador de tarefas assíncronas via Redis             |
| **yt-dlp**      | Ferramenta de linha de comando para baixar vídeos        |
| **Deno**        | Runtime JS que o yt-dlp usa para resolver o desafio `n` do YouTube |
| **Redis**       | Fila de tarefas e Pub/Sub para comunicação de eventos    |
| **SSE (Server-Sent Events)** | Comunicação em tempo real do backend para o frontend |

---

## 🧱 Funcionalidades

- 🔗 Download multiplataforma: cole o link e o sistema identifica a origem
- 🎚️ Escolha de qualidade com tamanho estimado
- 📶 Progresso em tempo real (percentual, velocidade, ETA) e cancelamento
- ⏳ Processamento assíncrono com fila (Redis + Asynq)
- 📥 Execução de download com yt-dlp
- 🔁 Envio de status em tempo real via SSE (`GET /downloads/:id/stream`)
- 📊 Histórico e status de tarefas (em breve com banco de dados)
- 🔌 API para integrar outros sistemas: chave de API, cota, limites por IP,
  webhooks assinados e auditoria por integração
- ⚙️ Pronto para escalar com múltiplos workers

---

## 📂 Estrutura de Pastas

```bash
yt-dlp-downloader/
├── cmd/
│   ├── server/     # API HTTP com Gin
│   ├── worker/     # Worker Asynq que processa os downloads
│   └── browser/    # Serviço de navegador remoto (contas do YouTube)
├── internal/
│   ├── server/     # Handlers, rotas e middlewares
│   ├── jobs/       # Processadores das tarefas assíncronas
│   ├── tasks/      # Definições de tarefas e payloads
│   ├── db/         # Código gerado pelo sqlc
│   ├── browser/    # Cliente do serviço de navegador
│   ├── browserd/   # Implementação do serviço de navegador
│   ├── media/      # Contrato de download; media/ytdlp é a única parte que
│   │               # conhece o binário
│   ├── providers/  # Monta o conjunto de providers
│   ├── ytaccounts/ # Sessões das contas entregues ao downloader
│   ├── integrations/ # Chaves de API, webhooks e controle de abuso dos
│   │                 # sistemas integrados
│   └── libs/       # Storage (R2) e streams do Redis
├── docs/
│   └── api-integracoes.md  # Guia da API consumida por outros sistemas
├── compose.dev.yaml    # Ambiente local completo
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

---

---

## 🌐 Download multiplataforma

O usuário cola qualquer link público, o sistema identifica a plataforma sozinho,
mostra título, miniatura, duração e as qualidades disponíveis, e baixa com
progresso em tempo real.

```
URL → normalização → detecção de plataforma → provider → metadados
    → escolha de qualidade → job → download com progresso → R2
```

### Arquitetura

O núcleo não sabe o que é yt-dlp. Quem sabe é uma única implementação, e trocá-la
não alcança handlers, jobs, banco ou frontend.

| Pacote | Responsabilidade |
|---|---|
| `internal/media` | Contrato (`Provider`), tipos, erros de domínio, detecção de plataforma, normalização de URL |
| `internal/media/ytdlp` | A **única** parte que conhece o binário: argumentos, progresso, erros |
| `internal/providers` | Monta o conjunto de providers a partir da configuração |

A interface é curta de propósito — formatos vêm dentro dos metadados (pedi-los à
parte dobraria a ida à plataforma) e o cancelamento é `context.Context`, que é
como Go já expressa isso:

```go
type Provider interface {
    Name() string
    CanHandle(*url.URL) bool
    Metadata(ctx, *url.URL) (*Metadata, error)
    Download(ctx, Request, ProgressFunc) (*Result, error)
    Health(ctx) Health
}
```

**Plataforma e provider são conceitos distintos**, e por isso são colunas
distintas no banco. Hoje todas as plataformas são atendidas pelo yt-dlp; somar um
provider dedicado no futuro é acrescentar uma linha em
`internal/providers/providers.go`:

```go
return media.NewRegistry(instagramProvider, tiktokProvider, ytdlpProvider)
```

O registry tenta na ordem, e o yt-dlp fica por último como fallback geral. O
fallback só dispara para falhas de **disponibilidade**: vídeo privado, ao vivo ou
removido não reentra, porque o próximo provider chegaria à mesma conclusão.

### Formatos

`normalizarFormatos` fica com a melhor opção por altura de vídeo, mais a melhor
faixa de áudio — o yt-dlp devolve trinta ou mais variações do mesmo formato, e
oferecer todas só atrapalharia quem escolhe.

**Formatos segmentados (HLS/DASH) contam.** Essa função já os descartava, com a
justificativa de que "não geram um arquivo final utilizável" — o que é falso: o
yt-dlp baixa os fragmentos e remuxa em MP4 normal. O efeito era grave e
silencioso: Vimeo, Dailymotion, Pinterest, Reddit e X servem **só** HLS, então a
lista de formatos vinha vazia, o download caía num seletor genérico e terminava
em "o formato escolhido não está disponível" — um erro sobre a escolha do
usuário para uma causa que não tinha nada a ver com ela. Entre dois formatos da
mesma altura o arquivo único ainda ganha do fragmentado, por informar o tamanho
exato e dispensar a remuxagem.

**Os ids do yt-dlp não são estáveis** entre duas extrações do mesmo conteúdo: em
HLS eles carregam o CDN sorteado na hora. Por isso o download guarda também a
altura pretendida (`format_height`) e o `-f` é uma cadeia que degrada em vez de
desistir:

```
<id>+bestaudio / <id> / bestvideo[height<=H]+bestaudio / best[height<=H] / ... / best
```

O yt-dlp para na primeira alternativa que casar. Se o id sumiu, cai na mesma
resolução por outro caminho; na pior das hipóteses entrega o melhor disponível,
em vez de falhar.

### Segurança da URL

`media.NormalizeURL` é a fronteira entre entrada não confiável e o resto do
sistema. Recusa esquema que não seja http(s) (fecha `file:///etc/passwd`),
endereços de rede interna (`localhost`, faixas privadas, `169.254.169.254`,
`redis:6379`), caracteres de controle e URL começando com hífen. Descarta
credenciais embutidas e o fragmento antes de gravar.

A detecção de plataforma usa um mapa de domínios registráveis com regra de
sufixo, e não `strings.Contains` — `youtube.com.site-falso.net` não é o YouTube.

O comando nunca é montado por concatenação: os argumentos vão estruturados para o
`exec`, sem shell, e `--` separa as opções da URL. O `format_id` e o nome do
arquivo passam por expressão regular antes de virarem argumento.

### Progresso

O progresso sai do `--progress-template` do yt-dlp com um prefixo próprio,
trafega por SSE (o mesmo Redis Stream do resto) e é gravado no banco a cada 10 s
— só o suficiente para a tela reabrir no meio de um download.

**O yt-dlp reporta por FAIXA, não por download.** Acima de 720p vídeo e áudio
vêm separados, e ele baixa um de cada vez reiniciando a contagem: a barra subia
até o fim, voltava para zero e subia de novo. `internal/media/ytdlp/progresso.go`
agrega as faixas em um progresso único:

- o id do formato (`%(info.format_id)s` no template) marca a troca de faixa; sem
  ele, a contagem voltando para trás é o sinal de reserva;
- os bytes das faixas concluídas continuam contando, então o numerador só cresce;
- o denominador é o tamanho estimado do conteúdo inteiro — vídeo **mais** a
  faixa de áudio que será somada a ele (`tamanhoEstimado`, gravado em
  `total_bytes` na criação). Sem isso a barra travaria perto do fim;
- há trava para trás: se a estimativa era curta e o total cresce no meio, o
  percentual segura em vez de cair;
- a fase de download para em 99%. Os 100% pertencem ao desfecho do job, depois
  da junção das faixas — anunciá-los antes é o que fazia a barra parecer
  concluída e então recomeçar.

A tela mostra percentual e bytes, e só. Velocidade e ETA são medidos por faixa e
reiniciavam junto, contradizendo a barra.

### Cancelamento

Cancelar **nunca falha por corrida de status**. O filtro de status vive no `SET`
da query, não no `WHERE`: clicar em cancelar meio segundo depois de o download
falhar sozinho antes devolvia 409 e a tela cuspia "este download não pode mais
ser cancelado" — um erro sobre algo que o usuário não provocou nem podia evitar.
Agora a linha sempre volta, quem já terminou mantém o status que tinha, e 404
fica reservado para um id que não é daquele usuário.

O pedido grava uma chave no Redis **antes** do UPDATE — gravar depois abriria
uma janela em que o download consta cancelado e segue baixando. O worker observa
essa chave a cada 2 s **independente do progresso**: um download que está
resolvendo ou pós-processando não emite atualização, e depender do callback
deixaria esses momentos sem resposta. O processo roda em grupo próprio
(`Setpgid`), então o SIGKILL alcança o ffmpeg junto — sem isso ficaria um órfão
escrevendo em disco.

E cancelar não deixa sobra:

| Momento do cancelamento | O que acontece |
| --- | --- |
| Na fila | O job vê `CANCELED` e nem começa |
| Baixando | Contexto cancelado, grupo de processos morto, diretório temporário removido |
| Entre o fim do download e o upload | Nova checagem antes de encaminhar; arquivo descartado |
| Upload na fila | O job de upload verifica o status, apaga os arquivos locais e não sobe nada |

A API publica o evento `CANCELED` na hora, sem esperar o worker perceber, e a
tela marca o card otimisticamente — 2 s com o botão ainda aceso pareciam clique
perdido.

### Arquivos temporários

Cada download tem um diretório próprio, removido no fim — sucesso, falha ou
cancelamento. Um container derrubado no meio não roda essa limpeza, então o
worker também varre sobras na subida e de hora em hora. Durante o
desenvolvimento isso recuperou 2,4 GB de uma tacada.

### Erros

A saída bruta do yt-dlp **nunca** chega ao usuário. `internal/media/ytdlp/errors.go`
traduz para erros de domínio (`ErrContentPrivate`, `ErrGeoBlocked`,
`ErrLiveContent`, `ErrRateLimited`…), a tela recebe a mensagem e um código
estável, e o detalhe técnico fica no log. Erros de conteúdo não são
reenfileirados: um vídeo privado continuará privado na terceira tentativa.

### Duas camadas de mensagem

**Regra dura: nada da nossa infraestrutura chega ao cliente final.** Ele não
pode saber que existe servidor, provider, sessão gerenciada ou plataforma
recusando alguma coisa. Para ele a falha é do produto, e a única informação útil
é o que fazer a seguir.

Por isso cada erro de domínio tem duas mensagens, em tabelas separadas:

| | Quem lê | Exemplo (`ErrBlocked`) |
| --- | --- | --- |
| `UserMessage` | log e super admin | "a plataforma recusou o acesso a partir deste servidor" |
| `PublicMessage` | **todo mundo** | "Não foi possível baixar este conteúdo agora. Tente novamente mais tarde." |

`PublicMessage` é a única que pode aparecer em resposta de API, no histórico ou
num evento de tempo real. Erros sobre o **conteúdo** (privado, ao vivo,
indisponível) continuam explícitos: não revelam nada nosso e são justamente o
que a pessoa precisa saber para parar de tentar.

Os códigos de erro seguem a mesma regra: `blocked`, `network` e
`provider_unavailable` chegam colapsados em `unavailable` para quem não é super
admin — a distinção entre eles é nossa. `internal/media/vazamento_test.go`
verifica termo a termo que nenhuma mensagem pública cita interno, e o teste
falha antes de a frase errada chegar a um cliente.

O padrão de `respostaDeErro` é o mais restrito: qualquer caminho que esqueça de
identificar o usuário cai no ramo público, nunca no que expõe interno. A tela
tem a mesma barreira do seu lado — não desenha os campos de diagnóstico para
quem não é super admin, para que um erro futuro de um dos lados não vire
vazamento sozinho.

### API e worker precisam ser reconstruídos JUNTOS

Resolver e baixar rodam em imagens diferentes. Quando só uma é reconstruída, o
sintoma é enganoso: **o link resolve e o download falha** — a API tem a
dependência nova e o worker não. Foi o que aconteceu com o `curl-cffi`.

Duas coisas tornam isso visível em vez de misterioso:

- o painel de providers mostra as dependências **por etapa**, então um
  `impersonação (curl-cffi): ausente` na etapa de Download aponta direto para a
  imagem do worker;
- quando um bloqueio acontece numa plataforma que exige impersonação e ela não
  existe naquele processo, o detalhe do erro diz isso em vez de só "403".

### Por que falhou (só para o super admin)

Esconder o motivo de **quem opera** transformava todo problema em adivinhação:
um 403 da plataforma, um provider quebrado e um vídeo removido viravam a mesma
frase na tela. Duas informações são anexadas à resposta de erro **apenas para o
super admin**:

- `detail` — o resumo do stderr (4 linhas, 500 caracteres, avisos removidos);
- `session` — se a conta gerenciada chegou a ser usada, e qual. É a primeira
  pergunta quando algo falha numa plataforma que exige login.

O mesmo vale para downloads já concluídos: `error_detail` guarda o motivo
técnico ao lado da mensagem pública, e a listagem do histórico só o devolve (e
só devolve o nome do provider) quando quem pede é super admin. Sem isso, uma
falha de download exigia caçar a linha no log do container — justamente o que o
histórico deveria evitar.

Para qualquer outro papel a API não envia nenhum dos dois; a mensagem de domínio
continua sendo tudo que o usuário comum vê.

A tabela em `errors.go` cobre também o que antes caía no genérico:

| Resposta da plataforma | Erro de domínio | O que significa |
| --- | --- | --- |
| 403, 401, 451, "blocked", captcha, 5xx | `ErrBlocked` | A plataforma recusou **este servidor**, não o conteúdo. Típico de IP de datacenter em Reddit, X e Pinterest, que atendem normalmente uma conexão residencial. A saída é uma conta autenticada |
| connection reset/refused, DNS, timeout de leitura | `ErrNetwork` | Não chegou a haver resposta; não adianta culpar o conteúdo |

A ordem da tabela importa: 429 e "login required" vêm antes, porque são causas
mais específicas para respostas da mesma família.

### Impersonação de navegador

Reddit, X e Pinterest respondem `HTTP Error 403: Blocked` já na **primeira**
requisição, antes de olhar cookie nenhum — por isso ter conta autenticada não
resolvia sozinho. O bloqueio é por fingerprint de TLS, e a resposta é imitar o
handshake de um navegador real (`--impersonate`, via o extra `curl-cffi`).

É por plataforma, não global: o YouTube funciona sem isso, e impersonar onde não
é preciso só adiciona uma dependência ao caminho crítico. Sob impersonação o
`--user-agent` configurado é **suprimido** — o yt-dlp já envia o do navegador
imitado, e sobrescrevê-lo deixaria handshake e cabeçalho contando histórias
diferentes, que é por si só sinal de automação.

O suporte é sondado uma vez por provider (`--list-impersonate-targets`) porque
`--impersonate` com alvo indisponível é **erro fatal**, não aviso: sem a
sondagem, uma imagem construída sem `curl-cffi` quebraria todo download dessas
plataformas. O painel de providers mostra se está ativa.

Se mesmo assim a plataforma recusar, o bloqueio é da faixa de IP e nenhuma
mudança na aplicação resolve. Aí entra o proxy — e ele é dividido em dois.

### Proxy só na extração (`RESOLVE_PROXY_URL`)

Um proxy residencial é vendido por **volume**, e as duas coisas que o yt-dlp faz
têm ordens de grandeza diferentes:

| Fase | O que trafega | Medido |
| --- | --- | --- |
| Extração de metadados | JSON com títulos, formatos e URLs | **~31 KB** |
| Download da mídia | os bytes do vídeo | **5 a 100+ MB** |

Só a **extração** é bloqueada pelo IP de datacenter; o CDN que serve os
fragmentos não liga para isso. Então a extração sai pelo proxy e a mídia vai
direto:

1. o worker roda `--dump-single-json` pelo `RESOLVE_PROXY_URL` e grava a
   extração em `extracao.info.json` dentro do diretório do download;
2. o download roda com `--load-info-json` e **sem proxy** — com a extração em
   mãos o yt-dlp não volta à plataforma, vai direto aos bytes.

Medido de ponta a ponta com um proxy que conta bytes, baixando um vídeo do X
pelo código do projeto: **62 KB pelo proxy, 5,5 MB direto**. Com 1 GB de cota
isso é a diferença entre ~100 vídeos e mais de dez mil.

`RESOLVE_PROXY_URL` é independente de `PROXY_ENABLED`/`PROXY_URL`, que continuam
sendo o proxy geral para **todo** o tráfego. Ligar os dois anula a economia, e o
script de build avisa quando isso acontece.

Falhar na extração pelo proxy não derruba o download: ele cai no caminho de uma
fase só, que é o de sempre. Um proxy mal configurado degrada, não quebra.

Um download é reenfileirado no máximo **3 vezes** (`MaxTentativasDownload`); o
padrão do asynq, 25, viraria duas dezenas de tentativas seguidas contra quem já
disse não, que é o tipo de insistência que aprofunda um bloqueio em vez de
contorná-lo. Erros de conteúdo já não repetem nenhuma vez.

### Diagnóstico

`/admin/providers` mostra o estado do mecanismo — versão do yt-dlp, ffmpeg e
runtime JavaScript, e quais estão ausentes.

A tela separa **duas etapas**, porque elas rodam em processos e imagens
diferentes:

| Etapa | Processo | ffmpeg | deno |
| --- | --- | --- | --- |
| Resolução de metadados | API (`Dockerfile.server`) | não usado — só executa `--dump-single-json` | usado, opcional |
| Download | Worker (`Dockerfile.worker`) | **obrigatório** — junta faixas separadas e converte para MP3 | usado, opcional |

Cada dependência é classificada em três estados, não dois: **obrigatória**
(sem ela a etapa não roda), **opcional** (a etapa a executa, e a ausência
degrada o resultado) e **não usada nesta etapa** (pertence à outra). Um booleano
só confundia os dois últimos, e o painel chegou a dizer que o `deno` não era
usado na resolução — ele é: sem o runtime JavaScript, requisições com cookies de
conta autenticada falham nas duas etapas.

O painel roda na API, então executar o diagnóstico ali não diria nada sobre o
worker. Em vez disso o worker publica seu relatório no Redis
(`media:health:downloader`, TTL de 10 min, republicado na subida e a cada 5 min)
e a API o lê. O TTL é o que faz um worker parado aparecer como *sem relatório*
em vez de continuar saudável para sempre.

Isso também é o motivo de a API não trazer ffmpeg: `Role` no `ytdlp.Config`
distingue os dois papéis, e cobrar a dependência do processo que nunca a executa
reportaria como falha algo que nunca quebrou.

Para diagnóstico pela linha de comando existe `cmd/mediaprobe`, que fica fora
das imagens de produção:

```sh
go run ./cmd/mediaprobe health
go run ./cmd/mediaprobe "https://youtu.be/..." "https://vimeo.com/..."
go run ./cmd/mediaprobe download "https://youtu.be/..." "137" 5s
```

### Suporte por plataforma

Reconhecer a URL e conseguir baixar são coisas diferentes:

| Situação | Exemplos | O que acontece |
| --- | --- | --- |
| Funciona anônimo | YouTube, TikTok, LinkedIn, Twitch | Baixa direto |
| Só HLS/DASH | Vimeo, Dailymotion, Pinterest, Reddit, X | Baixa direto — o yt-dlp busca os fragmentos e remuxa em MP4 |
| Precisa de impersonação | Dailymotion, Vimeo | O extra `curl-cffi` do yt-dlp entrega o TLS fingerprint de navegador que essas plataformas exigem. Sem ele: `none of these impersonate targets are available` |
| Precisa de conta | Vimeo (sempre), Pinterest / Reddit / X (do IP do datacenter) | Cadastre a conta da plataforma no painel |
| Sem extractor | Kwai | Recusado na hora, dizendo o nome da plataforma |

O Kwai é reconhecido pela URL, mas o yt-dlp não tem extractor para ele. Deixar o
caminho genérico tentar custava dezenas de segundos para terminar em
`Unsupported URL`; `Platform.TemSuporte()` recusa antes de sair da máquina, e o
painel de providers mostra essas plataformas riscadas.

### Compatibilidade

`POST /v1/downloads` continua aceitando `{type, url}` e restrito ao YouTube, como
sempre foi — mas agora **delega** ao mesmo caminho do downloader
multiplataforma, em vez de manter uma segunda implementação.

## 🛠️ Painel de administração

O Super Admin tem uma área própria em `/admin`, com quatro telas:

| Tela | O que faz |
|---|---|
| **Dashboard** | Downloads por período, volume transferido, armazenamento e totais da plataforma |
| **Usuários** | Listagem paginada com busca e filtros; criar, editar, bloquear, remover e redefinir senha |
| **Downloads** | Histórico global, filtrável por usuário, status e texto; remoção com o arquivo saindo do bucket |
| **YouTube** | Contas gerenciadas usadas pelo downloader (seção acima) |

### Períodos e gráficos

Hoje, ontem, 7, 15, 30, 90 dias, 12 meses e intervalo personalizado. A
granularidade é derivada do tamanho do intervalo — hora, dia, semana ou mês —
porque 90 dias em barras de hora dariam 2160 pontos e um gráfico ilegível.

Os baldes são truncados no fuso **America/Sao_Paulo**, o mesmo do limite diário.
Em UTC o "dia" do gráfico começaria às 21h do dia anterior e não bateria com o
contador que o usuário vê na tela.

### Armazenamento

A coluna `downloads.file_size_bytes` é gravada pelo worker logo após o upload,
medida antes de o arquivo local ser apagado. Ela sustenta três números
diferentes:

- **Armazenado agora** — concluído, não removido e dentro da validade: é o que
  ocupa espaço no R2 neste momento;
- **Já liberado** — expirado ou removido;
- **Histórico total** — tudo que já passou pelo bucket.

Varrer o bucket a cada carregamento do painel seria lento e cobrado por
requisição; por isso a contabilidade vive no banco. Downloads anteriores a esta
coluna aparecem com tamanho zero — o arquivo deles já expirou, e contá-los como
armazenamento atual seria pior do que não contar.

### Senhas

Ao criar um usuário sem informar senha, o painel gera uma com `crypto/rand` e a
exibe **uma única vez**. O banco guarda apenas o hash, então não há como
consultá-la depois — só gerar outra. O mesmo vale para "Gerar nova senha" na
edição.

### Travas

- O super admin **não tem teto** de downloads por dia nem de tamanho por plano.
- Bloquear um usuário recusa o login na hora, sem emitir token.
- Remover é lógico: o histórico de downloads é preservado, e a conta perde o
  acesso imediatamente.
- Não é possível remover a própria conta, nem rebaixar/bloquear o **último**
  super admin ativo — isso trancaria todo mundo para fora do painel, e não há
  tela para desfazer.

## 🔐 Contas das plataformas (painel de Super Admin)

O downloader pode usar sessões autenticadas de contas controladas pelo
administrador, em vez de um arquivo de cookies mantido à mão. O login é sempre
**manual**: o sistema abre um navegador remoto, o Super Admin entra na conta e a
sessão fica salva no perfil persistente do Chrome. **Nenhuma senha é pedida,
exibida ou armazenada.**

### Por que não é só do YouTube

Duas coisas diferentes levam à mesma solução:

- **Plataformas que exigem login por natureza.** O Vimeo recusa até a *leitura*
  de metadados sem sessão: `the web client only works when logged-in`.
- **Plataformas que atendem o IP residencial e bloqueiam o do datacenter.**
  Pinterest, Reddit e X resolvem normalmente de uma máquina doméstica e são
  recusados no servidor. A conta é o que devolve o acesso.

Cada plataforma tem um perfil em `internal/browser/plataformas.go`: a tela de
login que o navegador abre, os domínios de cookie que podem sair do container,
os nomes que indicam sessão logada e se a *resolução* também precisa de conta.
Os cookies **nunca vazam entre plataformas** — a sessão do Vimeo não leva junto
os do Google.

O rodízio é **por plataforma**: uma conta do YouTube não autentica no Vimeo, e
emprestá-la só gastaria a conta errada.

Buscar cookies pode subir um Chrome headless sobre o perfil, e a resolução roda
dentro da requisição de quem colou o link. Por isso ela só empresta sessão onde
resolver anônimo comprovadamente falha (`MetadadosExigemSessao`); o YouTube
resolve bem sem conta e não paga esse custo. O download sempre usa a conta,
quando existe.

### Fluxo

```
Super Admin → Contas → Adicionar conta (escolhe a plataforma) → Abrir navegador
   → login manual na plataforma
   → o painel detecta a sessão sozinho e marca "Autenticada"
   → fechar navegador → sessão persistida no perfil
   → downloads passam a usar a sessão
   → se a sessão expirar, o painel avisa e oferece "Reautenticar"
```

Enquanto a tela do navegador remoto está aberta, o painel verifica a sessão a
cada 10 s e para assim que ela é reconhecida. Não é preciso clicar em nada
depois de fazer o login.

### Componentes

| Serviço | Responsabilidade |
|---|---|
| `advideo-backend` | Painel, autorização de Super Admin, proxy do WebSocket do navegador |
| `advideo-worker`  | Downloads e health check agendado das sessões |
| `advideo-browser` | **Novo.** Único processo que executa o Chrome, mantém os perfis e exporta os cookies |

`advideo-browser` roda com uma réplica, sem porta publicada e apenas na
`agent_network`. Ele existe separado porque a API tem duas réplicas (um
navegador aberto em uma seria invisível para a outra) e porque API e worker estão
limitados a 512M, insuficiente para um Chrome headful com Xvfb.

Dentro dele: `Xvfb` (tela virtual) + `openbox` (foco e popups do login em duas
etapas) + `google-chrome` headful + `x11vnc` preso em `127.0.0.1`. O painel
enxerga a tela por um cliente noVNC que fala com a API; a API repassa o
WebSocket para o serviço. O Chrome é headful de propósito: o login do Google
recusa navegadores headless.

### Segurança

- Toda rota administrativa exige `role = super_admin`, verificado no banco.
- O WebSocket usa um **ticket de uso único** com 60 s de validade, guardado no
  Redis e amarrado ao par (usuário, conta) — o token de acesso nunca vai para a
  query string.
- O `x11vnc` escuta apenas em `127.0.0.1` dentro do container e ainda exige uma
  senha aleatória por sessão.
- Os perfis ficam em `/data/profiles/<uuid>` com permissão `0700`. O
  identificador é validado como UUID, o que fecha path traversal.
- Cookies nunca chegam ao frontend, ao banco ou aos logs. O worker recebe o jar
  em um arquivo `0600` temporário, apagado ao fim do download.

### Configuração

Use o mesmo `BROWSER_SERVICE_TOKEN` nos três serviços (`openssl rand -hex 32`).
Veja `api/.env.example`. Para criar o primeiro administrador, cadastre o usuário
normalmente e defina `SUPER_ADMIN_EMAIL` na API — ele é promovido no start.

**Sandbox do Chrome:** o sandbox precisa criar user namespaces, que o seccomp
padrão do Docker bloqueia. Em Compose isso se resolve com
`api/docker/chrome-seccomp.json` (já configurado em `compose.dev.yaml`), que
mantém todo o bloqueio padrão e libera apenas `clone`/`unshare`. O Swarm ignora
`security_opt`, então lá use `BROWSER_DISABLE_SANDBOX=true` — preferível a
conceder `CAP_SYS_ADMIN`, que seria bem pior para o host.

### Deploy

```bash
docker stack deploy -c docker-stack/deploy-browser.yml advideo-browser
```

O volume `advideo_browser_profiles` guarda as sessões e é o que faz elas
sobreviverem a restart, redeploy e recriação do container.

### Rodízio entre contas e failover

Quando existe mais de uma conta autenticada, os downloads se alternam entre
elas. A escolha é feita em uma única instrução no banco:

```sql
UPDATE ... WHERE id = (SELECT ... ORDER BY priority, last_used_at NULLS FIRST
                       LIMIT 1 FOR UPDATE SKIP LOCKED)
```

- **Escopo:** a seleção filtra por plataforma. A sessão do Vimeo não serve para
  baixar do Reddit.
- **Rodízio:** vence a conta usada há mais tempo, então o uso se distribui
  sozinho. `priority` (menor = preferida) permite manter uma conta principal e
  outras de reserva; com prioridades iguais o rodízio é circular.
- **Quando a conta NÃO sai do rodízio:** bloqueio de IP, limite de requisições,
  5xx e queda de rede não são culpa da conta. Essas causas têm precedência sobre
  qualquer palavra de login na mensagem, porque várias plataformas misturam as
  duas ideias na mesma linha — o X responde a um IP bloqueado com um texto que
  também fala em fazer login.

  Sem essa distinção o efeito era uma cascata: a primeira falha passageira tirava
  do rodízio uma conta perfeitamente boa, a tentativa seguinte ia **sem cookie
  nenhum** e falhava por outro motivo, e a conta aparecia "desconectando"
  sozinha. O administrador clicava em "Verificar sessão", ela voltava — porque a
  sessão nunca tinha deixado de valer.
- **Concorrência:** o `SKIP LOCKED` faz dois workers simultâneos pegarem contas
  diferentes em vez de disputarem a mesma linha, e o uso é registrado na mesma
  operação da seleção.
- **Failover imediato:** se a sessão escolhida não tiver cookies do Google, a
  conta é marcada como `REQUER AUTENTICAÇÃO` e a próxima é tentada na mesma
  requisição (até 5 contas).
- **Failover pelo veredito do YouTube:** se o yt-dlp responder que a sessão foi
  recusada, a conta sai do rodízio na hora. Na consulta de metadados a troca é
  imediata; no download o asynq reenfileira a tarefa e a próxima tentativa já
  escolhe outra conta.
- **Volta automática:** o health check a cada 15 minutos reavalia as contas em
  `REQUER AUTENTICAÇÃO` e devolve ao rodízio as que voltarem a funcionar.

Restrições de conteúdo (vídeo privado, exclusivo para membros, bloqueio
regional) **não** tiram a conta do rodízio: elas não indicam sessão inválida.

Isso é distribuição entre as contas legítimas do administrador e resiliência a
sessão expirada — não há rotação de IP, proxy ou identidade para contornar
limites do YouTube.

### Runtime JavaScript (obrigatório)

As imagens que executam o yt-dlp (`Dockerfile.server`, `Dockerfile.worker` e
`Dockerfile.dev`) instalam o **Deno**. Ele não é opcional quando existe conta
gerenciada: com cookies de uma conta autenticada e sem runtime JS, o YouTube
recusa a extração com `ERROR: The page needs to be reloaded`. Sem conta o
download ainda funciona, mas com menos formatos disponíveis.

### Health check

O worker verifica as sessões a cada 15 minutos: lê os cookies do perfil e, se
houver cookies de sessão, faz uma única requisição ao YouTube. Sessão inválida
vira `REQUER AUTENTICAÇÃO` no painel. Uma falha do serviço de navegador vira
`ERRO`, e não um pedido de login — a sessão salva pode estar intacta.

---

## 🚀 Produção no servidor ARM (Docker Swarm + Portainer)

O servidor é **ARM (aarch64)** e **não há registry**: as imagens são construídas
no próprio servidor, ficam locais com a tag `:latest`, e a stack é aplicada pelo
Portainer.

```sh
cp docker/builder/stack.env.example docker/builder/stack.env
chmod 600 docker/builder/stack.env
# preencha o arquivo
sh docker/builder/build-images.sh
```

O script valida tudo **antes** do primeiro build e recusa configuração errada:
chave PASETO com tamanho inválido, senha com caractere que quebra a URL do
Postgres, site key de teste do Turnstile, proxy ligado sem URL. Ele também
renderiza o `docswarm.yaml` e reprova qualquer URL com variável não substituída
— a interpolação do Compose não é recursiva, e uma URL montada pela metade só
apareceria quando o container subisse. Descobrir isso
depois de vinte minutos baixando o Chrome é uma espera que não se paga.

Cinco imagens: `advideo-migrate`, `advideo-api`, `advideo-worker`,
`advideo-browser` e `advideo-app`. Nenhum Dockerfile fixa `GOARCH`, e tanto o
repositório do Chrome quanto o download do Deno resolvem a arquitetura em tempo
de build — as imagens saem corretas para a máquina onde o script roda.

Depois de todo rebuild, **incremente `DEPLOY_REVISION`**. As imagens são
`:latest` e locais; sem essa mudança o Swarm conclui que nada mudou e mantém o
código velho no ar, com o build tendo terminado sem um único erro.

O passo a passo completo está em [`docker/builder/README.md`](docker/builder/README.md).

### Duas coisas que costumam morder

**O front carrega os domínios dentro do bundle.** `API_DOMAIN`, `APP_DOMAIN`,
`BUCKET_HOST` e `TURNSTILE_SITE_KEY` viram texto dentro do JavaScript no build.
Trocar qualquer um deles exige `sh docker/builder/build-images.sh app` —
atualizar a stack sozinho não muda um bundle já gerado.

**Em Swarm o sandbox do Chrome fica desligado.** O `security_opt` é ignorado por
`docker stack deploy`, então não há como entregar o perfil seccomp que libera
user namespaces. Manter o sandbox exigiria `CAP_SYS_ADMIN`, que é bem pior para
o host do que desligá-lo dentro de um container de propósito único, sem root e
sem porta publicada. O `build-images.sh` recusa buildar com
`BROWSER_DISABLE_SANDBOX` diferente de `true`.

Quando o sandbox está desligado, o serviço passa `--test-type` junto. Sem ele o
Chrome desenha uma faixa amarela — *"You are using an unsupported command-line
flag: --no-sandbox"* — no topo de toda janela: ela rouba ~56px da tela remota e
alarma quem só está fazendo login, sendo que o aviso é para quem opera e já está
dito aqui. A flag não é de automação: `navigator.webdriver` continua `false`, não
há `--enable-automation`, e o login do Google segue normal.

### Deploy antigo (x86, ghcr.io)

`docker-stack/deploy-*.yml` é o caminho anterior, com imagens publicadas no
GitHub Container Registry pelos workflows em `.github/workflows/`. Os dois não
devem conviver no mesmo servidor.

## 🔌 Integrações (a API consumida por outros sistemas)

A mesma engrenagem que atende a tela — mesma fila, mesmos providers, mesmo
armazenamento — exposta para ser consumida por programa, com autenticação por
chave de API em vez de login.

**Guia completo:** [`docs/api-integracoes.md`](docs/api-integracoes.md).
**Referência viva:** `GET /docs` e `GET /openapi.yaml`, servidos pela própria
API (`DOCS_ENABLED`).

### Uma integração É uma conta

Esta é a decisão que governa todo o resto. Cada integração possui uma linha em
`users` com `kind = 'service'`, e os downloads dela usam a coluna
`downloads.user_id` de sempre.

Sem isso, cada recurso já existente — cota diária, histórico paginado, expiração
de arquivo, contabilidade de armazenamento, eventos de tempo real, painel de
downloads — precisaria de um segundo caminho que fizesse a mesma coisa por um
identificador diferente, e cada correção futura teria de ser aplicada duas
vezes. Foi o mesmo raciocínio que fez a rota antiga de download delegar à nova
em vez de duplicar a lógica.

O que a conta de serviço NÃO compartilha é a forma de autenticar: ela não tem
senha utilizável (o hash guardado não é um bcrypt válido, então a comparação
falha para qualquer entrada), não aparece na tela de Usuários e é barrada no
`/auth/login` e no middleware de token. Quem prova identidade é a chave de API,
com a lista de IPs e o limitador que vêm junto.

### As rotas de download são literalmente as mesmas

`POST /v1/integration/downloads` e a criação da tela chamam a mesma função. O
middleware da chave de API deixa a conta de serviço no contexto pela MESMA chave
que o login usa, e os handlers não sabem qual dos dois autenticou — só o formato
da resposta difere.

Esse foi o motivo de um refactor no caminho: os handlers de download liam o
payload do token e iam ao banco buscar o usuário de novo. Agora leem o usuário
que o middleware já carregou, o que removeu uma consulta repetida por requisição
e um `MustGet` que derrubaria o processo em rota sem autenticação.

### Webhooks se penduram no stream que já existe

O worker já publica cada transição de status em um stream do Redis, que a API
consome para alimentar o SSE da tela. O despachante de webhooks se pendura no
**mesmo** consumidor, em vez de pedir que cada job avise os webhooks.

`internal/jobs` continua sem nenhuma noção de integração: qualquer transição que
apareça na tela chega ao sistema integrado pelo mesmo caminho, e um estado novo
no futuro não precisa ser publicado em dois lugares. Um segundo consumidor criaria
um grupo concorrente no mesmo stream, e cada evento iria para apenas um dos dois
destinos — metade das atualizações deixaria de aparecer na tela.

A entrega roda no **worker**, em fila própria: ela depende de um servidor de
terceiro responder, e um endpoint lento na mão de um cliente não pode ocupar nem
a goroutine que atende as requisições HTTP nem os slots que deveriam estar
baixando vídeo.

### O que protege a plataforma

Quatro controles, porque cada um contém um abuso diferente:

| Controle | Onde vive | Contém |
|---|---|---|
| Chamadas por minuto, por CHAVE | Redis | Laço apertado derrubando a API |
| Downloads simultâneos | Postgres | Gastar a cota do dia toda de uma vez, ocupando o worker |
| Cota diária | Postgres | Consumo além do contratado |
| Lista de IPs autorizados | Postgres | Chave vazada sendo usada de qualquer lugar |

O limite por minuto é por chave, e não por integração: quando há várias chaves
(uma por ambiente, por exemplo), o consumo descontrolado de uma não derruba as
outras. Ele falha ABERTO se o Redis estiver fora — recusar toda chamada de toda
integração por causa de um componente auxiliar transformaria degradação em
indisponibilidade total, e a cota e os simultâneos continuam valendo porque
vivem no Postgres.

### A chave de API

Guardada como **hash SHA-256**, nunca em claro. O valor completo existe uma
única vez: na resposta que a criou. Um vazamento do banco não entrega acesso às
integrações, e nem quem opera o painel consegue recuperar a chave de um cliente
— só emitir outra.

SHA-256 e não bcrypt, ao contrário da senha de usuário, porque a origem do
segredo é outra: a chave é gerada por nós com 256 bits de `crypto/rand`, não
escolhida por uma pessoa. Não há senha fraca a proteger nem dicionário a
atrasar, e um hash deliberadamente lento entraria no caminho de CADA requisição
autenticada.

### SSRF: a URL do webhook é escolhida por quem cadastra

E é o servidor que faz a requisição. Sem cuidado, o campo de URL do painel é uma
sonda da rede interna. Três camadas:

1. **No cadastro** — só `https`, sem credencial na URL, e o host tem de resolver
   para endereço público. `169.254.169.254` (metadados da instância),
   `10.0.0.0/8`, `127.0.0.1`, `100.64.0.0/10` e afins são recusados.
2. **Na discagem** — o dialer confere o IP real na hora de conectar, fechando a
   janela do DNS rebinding (validar o nome no cadastro e ele passar a resolver
   para um endereço interno depois).
3. **Sem redirecionamento** — um endpoint que responde `302` para um endereço
   interno não é seguido.

`WEBHOOK_ALLOW_PRIVATE=true` desliga as três e existe SOMENTE para
desenvolvimento, onde o sistema integrado roda em localhost.

### Auditoria fora do caminho da resposta

Cada chamada gera uma linha em `integration_requests` — IP, rota, status, código
de erro, tempo —, gravada por escritores dedicados que consomem uma fila em
memória. Gravar de forma síncrona dobraria as idas ao Postgres por requisição.
O contrato é explícito: sob pressão extrema, perde-se REGISTRO, nunca
disponibilidade.

É essa tabela que responde, no painel, as perguntas que aparecem quando algo vai
mal: de qual IP veio, qual chave usou, qual rota, o que respondemos e quanto
demorou. Tentativas recusadas por IP ou por chave revogada TAMBÉM entram — são
justamente as que interessam numa investigação, e é por isso que a integração é
registrada no contexto assim que a chave é reconhecida, antes das checagens de
acesso.

A poda roda de madrugada (`INTEGRATION_LOG_RETENTION_DAYS`, padrão 30 dias). As
entregas que falharam sobrevivem três vezes mais tempo: é nelas que alguém vai
procurar o motivo, muito depois do dia em que aconteceram.

### Duas camadas de mensagem, de novo

O mesmo princípio que separa `error_message` de `error_detail` no download vale
aqui. Um sistema cliente não é mais confiável que um usuário comum:

- `provider` (o mecanismo que baixa) e `error_detail` (o motivo técnico, muitas
  vezes com saída de processo) **nunca** saem para uma integração;
- o **segredo de assinatura** do webhook não sai pela API, só pelo painel: se
  saísse por uma rota autenticada pela chave, um vazamento de chave viraria
  também um vazamento do segredo — e com ele daria para forjar eventos assinados
  para o endpoint do cliente;
- download de outra integração responde `404`, não `403`. Distinguir "não
  existe" de "existe e não é seu" seria um oráculo para enumerar os
  identificadores dos outros.

### Painel

**Administração → Integrações**, só para super admin. Cada integração tem abas
de visão geral, chaves de API, webhooks, entregas, requisições/IPs e downloads.

A aba de **entregas** é a que encerra a discussão mais comum de qualquer
integração por webhook — "vocês enviaram?" / "não recebi" — com o corpo exato do
POST, o que o endpoint respondeu e quantas tentativas foram feitas.

### Configuração

Ver o bloco de integrações em `api/.env.example`. Em produção, o essencial:

```env
PUBLIC_API_URL=https://api.seu-dominio.com   # usado nos links do payload
WEBHOOK_ALLOW_PRIVATE=false                  # NUNCA true em produção
DOCS_ENABLED=true
```

---

## 📌 Próximas Melhorias

- [ ] Armazenamento persistente no PostgreSQL
- [ ] Upload automático para armazenamento em nuvem
- [ ] Dashboard com histórico e progresso em tempo real
- [x] Autenticação e controle de acesso
- [x] Gerenciamento de contas do YouTube pelo Super Admin
- [x] API para integração com outros sistemas (chaves, cotas, webhooks e auditoria)

---

Desenvolvido com 💻 por **Alves** ✨
