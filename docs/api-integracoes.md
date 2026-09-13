# API de Integrações

Guia para integrar **outro sistema** ao downloader. Se você só quer a
referência das rotas, ela é servida pela própria API:

- `GET /docs` — documentação navegável
- `GET /openapi.yaml` — especificação OpenAPI 3.1, para Insomnia, Postman,
  Bruno, Swagger UI e geradores de cliente

Este documento é o texto ao redor: o **porquê** de cada decisão, o fluxo de
ponta a ponta e o que costuma dar errado.

---

## Sumário

- [O que é isto](#o-que-é-isto)
- [Começando em cinco minutos](#começando-em-cinco-minutos)
- [Autenticação](#autenticação)
- [O fluxo de um download](#o-fluxo-de-um-download)
- [Webhooks](#webhooks)
  - [Verificando a assinatura](#verificando-a-assinatura)
  - [Reenvio, ordem e duplicidade](#reenvio-ordem-e-duplicidade)
- [Limites e cotas](#limites-e-cotas)
- [Erros](#erros)
- [Referência rápida das rotas](#referência-rápida-das-rotas)
- [O painel (lado do operador)](#o-painel-lado-do-operador)
- [Como isto funciona por dentro](#como-isto-funciona-por-dentro)
- [Problemas comuns](#problemas-comuns)

---

## O que é isto

A mesma engrenagem que atende a interface web — mesma fila, mesmos provedores,
mesmo armazenamento — exposta para ser consumida por programa, com autenticação
por chave de API em vez de login.

Uma **integração** é uma conta de sistema. Ela tem:

- um identificador público (UUID) e uma ou mais chaves de API;
- cota diária de downloads, teto de simultâneos e limite de chamadas por minuto
  próprios;
- lista opcional de IPs autorizados;
- webhooks próprios;
- auditoria própria: toda chamada fica registrada com IP, rota, resposta e
  tempo.

Integrações são criadas **somente pelo super admin no painel**. Não existe
auto-cadastro: um sistema que pudesse criar o próprio acesso tornaria a cota e a
lista de IPs decorativas.

---

## Começando em cinco minutos

O caminho mais curto: peça um download, receba o webhook, baixe o arquivo.

### 1. Confirme a credencial

```bash
curl https://api.advideo.com.br/v1/integration/me \
  -H "Authorization: Bearer adk_live_SUA_CHAVE"
```

```json
{
  "integration": { "id": "9f1c7a32-…", "name": "CRM Comercial", "active": true },
  "limits": {
    "daily_downloads": 200,
    "max_concurrent_downloads": 5,
    "rate_limit_per_minute": 120,
    "max_file_size_bytes": 1073741824,
    "allowed_ips": ["203.0.113.10"]
  },
  "usage": {
    "daily_used": 37,
    "daily_remaining": 163,
    "downloads_active": 2,
    "resets_at": "2026-09-13T00:00:00-03:00"
  }
}
```

### 2. Peça o download

```bash
curl -X POST https://api.advideo.com.br/v1/integration/downloads \
  -H "Authorization: Bearer adk_live_SUA_CHAVE" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}'
```

Resposta `202 Accepted`:

```json
{
  "download": {
    "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "status": "PENDING",
    "title": "Palestra sobre sistemas distribuídos",
    "platform": "youtube",
    "format": "MP4",
    "links": { "self": "https://api.advideo.com.br/v1/integration/downloads/3fa85f64-…" }
  }
}
```

**202 e não 201**: o pedido foi aceito, o arquivo não existe ainda. Guarde o
`id`.

### 3. Espere o aviso

Com webhook cadastrado, você recebe um `POST` quando o download termina. Sem
webhook, consulte:

```bash
curl https://api.advideo.com.br/v1/integration/downloads/3fa85f64-… \
  -H "Authorization: Bearer adk_live_SUA_CHAVE"
```

### 4. Baixe o arquivo

```bash
curl https://api.advideo.com.br/v1/integration/downloads/3fa85f64-…/download-url \
  -H "Authorization: Bearer adk_live_SUA_CHAVE"
```

```json
{
  "url": "https://…assinada…",
  "expires_at": "2026-09-12T18:45:02Z"
}
```

A URL assinada **vale um minuto** e aponta direto para o armazenamento — os
bytes não passam pela API, então não há limite de tamanho nem timeout de
requisição no caminho. Não guarde essa URL: guarde o `id` e peça outra quando
precisar.

Se o seu ambiente não alcança o armazenamento diretamente, use
`GET /downloads/{id}/file`, que entrega os bytes pela própria API.

---

## Autenticação

Toda chamada exige a chave emitida no painel:

```http
Authorization: Bearer adk_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Se o seu ambiente reescreve o header `Authorization` (proxies corporativos às
vezes fazem isso), use `X-API-Key` com o mesmo valor. Os dois são equivalentes;
`X-API-Key` tem precedência quando ambos vêm.

### O identificador da integração (opcional, recomendado)

```http
X-Integration-Id: 9f1c7a32-4b8e-4a1d-9c3f-2e5b7d8a1f04
```

Ele **não substitui a chave**. Serve como trava contra o acidente clássico de
configuração: a chave de homologação apontada para a integração de produção. Se
os dois não corresponderem, a chamada é recusada com `integration_mismatch` em
vez de operar na conta errada.

### A chave aparece uma vez

Na resposta que a criou, e só ali. O banco guarda apenas o **hash** dela
(SHA-256): nem quem opera o painel consegue recuperá-la — só emitir outra.

Se vazou ou foi perdida, o painel tem duas ações:

- **Emitir** outra chave, mantendo a atual válida — é como se troca a credencial
  de um sistema em produção sem derrubá-lo: sobe a nova, atualiza o cliente,
  revoga a velha.
- **Regerar**, que emite a nova e revoga todas as anteriores no mesmo instante.

### IPs autorizados

Se a integração tiver lista configurada, chamadas de outros endereços recebem
`403 ip_not_allowed` — e ficam registradas na auditoria, com IP e horário. A
lista atual aparece em `GET /me`, para você poder diagnosticar do seu lado.

Lista vazia aceita qualquer origem. Preenchê-la é a diferença entre uma chave
vazada ser inútil e ser utilizável de qualquer lugar do mundo.

---

## O fluxo de um download

```
     POST /downloads
           │
           ▼
      ┌─────────┐   evento: download.queued
      │ PENDING │ ─────────────────────────────►
      └────┬────┘
           │  o worker pegou a tarefa
           ▼
    ┌────────────┐  evento: download.started
    │ PROCESSING │ ────────────────────────────►
    └──┬──────┬──┘  (e download.progress, se assinado)
       │      │
       │      └──────────────┐
       ▼                     ▼
 ┌───────────┐         ┌──────────┐  evento: download.failed
 │ COMPLETED │         │  FAILED  │ ───────────────────────►
 └─────┬─────┘         └──────────┘
       │ evento: download.completed
       │
       ▼  passada a validade
 ┌───────────┐  evento: download.expired
 │  EXPIRED  │ ─────────────────────────────►
 └───────────┘

 A qualquer momento antes do fim: POST /downloads/{id}/cancel → CANCELED
```

O arquivo fica disponível por tempo limitado. `expires_at` diz até quando;
depois disso o status vira `EXPIRED` e o arquivo é removido do armazenamento.
Precisa dele de novo? Peça o download outra vez.

### Escolhendo a qualidade

Sem `format_id`, entregamos a melhor qualidade disponível do tipo pedido
(`kind: "video"` → MP4, `kind: "audio"` → MP3). Para escolher:

```bash
# 1. Consulte os formatos (não consome cota diária)
curl -X POST https://api.advideo.com.br/v1/integration/media/resolve \
  -H "Authorization: Bearer adk_live_SUA_CHAVE" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}'

# 2. Use o id de um dos formatos devolvidos
curl -X POST https://api.advideo.com.br/v1/integration/downloads \
  -H "Authorization: Bearer adk_live_SUA_CHAVE" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "format_id": "137"}'
```

> **Os identificadores de formato não são estáveis.** A plataforma os regenera
> entre duas consultas do mesmo conteúdo — em HLS eles chegam a carregar o CDN
> sorteado na hora. Resolva e crie o download na mesma sequência. Se o
> identificador expirar no meio do caminho, a criação **cai automaticamente para
> a resolução mais próxima** em vez de falhar; um valor que nunca poderia ter
> existido é recusado com `format_unavailable`.

---

## Webhooks

Cadastrados no painel pelo super admin. Assim que o download muda de estado,
fazemos `POST` na URL cadastrada:

```http
POST /webhooks/advideo HTTP/1.1
Content-Type: application/json
User-Agent: adVideo-Webhooks/1.0
X-Advideo-Event: download.completed
X-Advideo-Delivery: 7c9e6679-7425-40de-944b-e07fc1f90ae7
X-Advideo-Timestamp: 1757700242
X-Advideo-Signature: v1=6f3a…hex…
X-Advideo-Attempt: 1
```

```json
{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "type": "download.completed",
  "created_at": "2026-09-12T18:44:02Z",
  "integration_id": "9f1c7a32-4b8e-4a1d-9c3f-2e5b7d8a1f04",
  "data": {
    "download": {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "COMPLETED",
      "title": "Palestra sobre sistemas distribuídos",
      "original_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
      "platform": "youtube",
      "format": "MP4",
      "quality_label": "1080p",
      "duration_seconds": 3120,
      "file_size_bytes": 284551168,
      "expires_at": "2026-09-13T18:44:01Z",
      "links": {
        "self": "https://api.advideo.com.br/v1/integration/downloads/3fa85f64-…",
        "download_url": "https://api.advideo.com.br/v1/integration/downloads/3fa85f64-…/download-url",
        "file": "https://api.advideo.com.br/v1/integration/downloads/3fa85f64-…/file"
      }
    }
  }
}
```

O objeto em `data.download` é **o mesmo** que `GET /downloads/{id}` devolve. Um
parser serve para os dois.

### Eventos

| Evento | Quando |
|---|---|
| `download.queued` | O pedido entrou na fila |
| `download.started` | O worker começou a baixar |
| `download.progress` | Andamento — **só com assinatura explícita** |
| `download.completed` | Arquivo pronto para buscar |
| `download.failed` | Falhou; veja `error_message` |
| `download.canceled` | Cancelado a pedido |
| `download.expired` | A validade passou e o arquivo foi removido |
| `download.retrying` | Nova tentativa após falha passageira |
| `ping` | Evento de teste, disparado do painel ou por `/webhooks/{id}/test` |

Um webhook sem lista explícita recebe **todos os de ciclo de vida**.
`download.progress` fica de fora por padrão: são muitos eventos por download
(espaçados em alguns segundos, quando ligados), e quem só quer saber que o
arquivo ficou pronto não deve ter de descartar dezenas de POSTs para achar o que
importa.

### Verificando a assinatura

**Obrigatório.** Sem isso, qualquer um que descubra a sua URL pode enviar
"download.completed" falsos.

Assinamos `"<timestamp>.<corpo cru>"` com HMAC-SHA256 usando o segredo do
webhook (`whsec_…`, visível no painel). Verifique **antes de processar**, sobre
o corpo **cru** — reserializar o JSON muda bytes e invalida a assinatura.

#### Node / Express

```js
const crypto = require("crypto");
const express = require("express");

const app = express();
const SEGREDO = process.env.ADVIDEO_WEBHOOK_SECRET;

// express.raw, não express.json: a assinatura é sobre os bytes originais.
app.post("/webhooks/advideo", express.raw({ type: "application/json" }), (req, res) => {
  const ts = req.header("X-Advideo-Timestamp");
  const recebida = req.header("X-Advideo-Signature") || "";

  // Recusa o que for antigo demais: sem isto, uma entrega capturada hoje pode
  // ser reenviada amanhã com a assinatura ainda válida.
  if (Math.abs(Date.now() / 1000 - Number(ts)) > 300) {
    return res.status(401).end();
  }

  const esperada =
    "v1=" +
    crypto.createHmac("sha256", SEGREDO).update(`${ts}.${req.body}`).digest("hex");

  // Comparação em tempo constante: um === vazaria, pelo tempo de resposta,
  // quantos bytes o atacante já acertou.
  const ok =
    esperada.length === recebida.length &&
    crypto.timingSafeEqual(Buffer.from(esperada), Buffer.from(recebida));

  if (!ok) return res.status(401).end();

  const evento = JSON.parse(req.body.toString("utf8"));

  // Responda rápido e processe depois: o timeout é de 20 segundos.
  res.status(202).end();
  processarEmBackground(evento);
});
```

#### PHP

```php
<?php
$segredo = getenv('ADVIDEO_WEBHOOK_SECRET');
$bruto = file_get_contents('php://input');
$ts = $_SERVER['HTTP_X_ADVIDEO_TIMESTAMP'] ?? '';
$recebida = $_SERVER['HTTP_X_ADVIDEO_SIGNATURE'] ?? '';

if (abs(time() - (int) $ts) > 300) {
    http_response_code(401);
    exit;
}

$esperada = 'v1=' . hash_hmac('sha256', $ts . '.' . $bruto, $segredo);

if (!hash_equals($esperada, $recebida)) {
    http_response_code(401);
    exit;
}

$evento = json_decode($bruto, true);
http_response_code(202);
```

#### Python / Flask

```python
import hashlib, hmac, os, time
from flask import Flask, request

app = Flask(__name__)
SEGREDO = os.environ["ADVIDEO_WEBHOOK_SECRET"].encode()

@app.post("/webhooks/advideo")
def receber():
    bruto = request.get_data()  # bytes originais, não request.json
    ts = request.headers.get("X-Advideo-Timestamp", "")
    recebida = request.headers.get("X-Advideo-Signature", "")

    if abs(time.time() - int(ts or 0)) > 300:
        return "", 401

    esperada = "v1=" + hmac.new(
        SEGREDO, f"{ts}.".encode() + bruto, hashlib.sha256
    ).hexdigest()

    if not hmac.compare_digest(esperada, recebida):
        return "", 401

    evento = request.get_json()
    return "", 202
```

#### Go

```go
func verificar(r *http.Request, segredo string) ([]byte, error) {
	bruto, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	ts := r.Header.Get("X-Advideo-Timestamp")
	segundos, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || math.Abs(float64(time.Now().Unix()-segundos)) > 300 {
		return nil, errors.New("timestamp inválido ou antigo")
	}

	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(bruto)
	esperada := "v1=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(esperada), []byte(r.Header.Get("X-Advideo-Signature"))) {
		return nil, errors.New("assinatura inválida")
	}
	return bruto, nil
}
```

### Como responder

Responda **2xx** (200, 202 e 204 servem) o mais rápido possível e processe
depois. O timeout é de **20 segundos por tentativa**.

| Sua resposta | O que fazemos |
|---|---|
| `2xx` | Entrega concluída |
| `408`, `429`, `5xx`, timeout, erro de rede | Reenviamos, com espera crescente, até 8 tentativas |
| Outros `4xx` | Entendemos como recusa e **não** reenviamos — o mesmo corpo produziria a mesma recusa |
| `410 Gone` | Paramos e **desativamos** o webhook |

Após **10 entregas abandonadas seguidas**, o webhook é desativado
automaticamente e o motivo aparece no painel. Cada entrega abandonada já esgotou
as 8 tentativas, o que leva horas — dez seguidas não é oscilação, é endereço
morto.

### Reenvio, ordem e duplicidade

**Não garantimos ordem nem entrega única.** Reenvios existem, e um evento mais
novo pode chegar antes de um mais velho. Duas defesas do seu lado resolvem:

1. **Idempotência**: guarde o `X-Advideo-Delivery` (igual ao `id` do corpo) e
   ignore o que já processou.
2. **Não retroceda**: compare o `status` recebido com o que você já tem e ignore
   regressões (`PROCESSING` chegando depois de `COMPLETED`).

Em caso de dúvida, `GET /downloads/{id}` é sempre a verdade atual.

Se o seu endpoint ficou fora do ar, `GET /downloads?status=COMPLETED` recupera o
que você não recebeu — é a rede de segurança do webhook.

### Testando

O painel tem um botão **Testar** em cada webhook, e a API tem a rota
equivalente:

```bash
curl -X POST https://api.advideo.com.br/v1/integration/webhooks/{id}/test \
  -H "Authorization: Bearer adk_live_SUA_CHAVE"
```

Dispara um evento `ping` para o endpoint cadastrado — valida URL e verificação
de assinatura sem esperar um download real. O resultado aparece em
`GET /v1/integration/deliveries`, com o que o seu endpoint respondeu.

> **O segredo de assinatura não sai pela API**, por decisão de segurança: se
> saísse por uma rota autenticada pela chave, um vazamento de chave viraria
> também um vazamento do segredo — e com ele daria para forjar eventos assinados
> para o seu endpoint. Ele é entregue pelo painel, a uma pessoa.

---

## Limites e cotas

Três controles independentes, cada um com uma reação diferente do seu lado:

| Controle | Erro | Status | O que fazer |
|---|---|---|---|
| Chamadas por minuto (por chave) | `rate_limited` | 429 | Respeitar `Retry-After` e continuar |
| Cota diária de downloads | `quota_exceeded` | 429 | Agendar para depois de `X-Quota-Reset` |
| Downloads simultâneos | `too_many_concurrent` | 429 | Esperar os atuais terminarem (~30 s) |
| Tamanho do arquivo | `file_too_large` | 400 | Escolher um formato menor |

Headers que acompanham as respostas:

```http
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 118
X-Quota-Limit: 200
X-Quota-Remaining: 163
X-Quota-Reset: 2026-09-13T00:00:00-03:00
Retry-After: 12
```

Usá-los dispensa chamar `GET /quota` para acompanhar o consumo.

**A cota diária reseta à meia-noite no fuso `America/Sao_Paulo`** — o mesmo
corte usado pelo contador que o usuário final vê. Em UTC, ela viraria às 21h.

Sobre o limite por minuto: além do teto que você contratou, há um teto de rajada
derivado dele (um décimo da cota por segundo, com piso de 5). Ele deixa você
disparar um punhado de chamadas em paralelo — o caso real de quem processa uma
fila — e impede gastar o minuto inteiro no mesmo instante.

---

## Erros

Toda resposta de erro tem a mesma forma:

```json
{ "error": "cota diária de downloads esgotada", "code": "quota_exceeded" }
```

**Trate `code`, nunca `error`.** A mensagem é para o humano que lê o log e pode
ser reescrita a qualquer momento; o código é contrato.

### Credencial e acesso

| Código | Status | Significado |
|---|---|---|
| `missing_credentials` | 401 | Nenhuma chave foi enviada |
| `invalid_api_key` | 401 | Chave desconhecida ou malformada |
| `key_revoked` | 401 | A chave foi revogada no painel |
| `key_expired` | 401 | A chave tinha validade e ela passou |
| `integration_mismatch` | 401 | `X-Integration-Id` não corresponde à chave |
| `integration_disabled` | 403 | A integração foi desativada ou removida |
| `ip_not_allowed` | 403 | Origem fora da lista de IPs autorizados |

### Limites

| Código | Status |
|---|---|
| `rate_limited` | 429 |
| `quota_exceeded` | 429 |
| `too_many_concurrent` | 429 |
| `file_too_large` | 400 |

### Requisição e recurso

| Código | Status | Significado |
|---|---|---|
| `invalid_request` | 400 | Corpo ou filtro inválido |
| `not_found` | 404 | Não existe **ou não é desta integração** |
| `download_not_ready` | 409 | O download ainda não terminou |
| `download_expired` | 410 | O arquivo expirou e foi removido |
| `internal_error` | 500 | Erro nosso |
| `service_unavailable` | 503 | Mecanismo de download indisponível |

> `not_found` responde igual para "não existe" e "existe, mas é de outra
> integração". É deliberado: distinguir os dois permitiria descobrir os
> identificadores de outra integração por tentativa.

### Sobre o conteúdo pedido

| Código | Status | Significado |
|---|---|---|
| `invalid_url` | 400 | URL não reconhecida |
| `unsupported_platform` | 404 | Plataforma não atendida |
| `content_unavailable` | 404 | Conteúdo removido ou inexistente |
| `content_private` | 422 | Conteúdo privado |
| `auth_required` | 422 | Exige login que não está disponível |
| `geo_blocked` | 422 | Restrito por região |
| `live_content` | 422 | Transmissão ao vivo |
| `format_unavailable` | 400 | `format_id` fora do vocabulário |
| `unavailable` | 403/502/503 | Indisponibilidade temporária do nosso lado |
| `timeout` | 504 | A plataforma demorou demais |
| `download_failed` | 400 | Falha genérica |

Os `422` são definitivos para aquele conteúdo: marque o item e não tente de
novo. `unavailable` e `timeout` valem uma nova tentativa mais tarde.

---

## Referência rápida das rotas

Todas sob `/v1/integration`, todas autenticadas por chave de API.

| Método | Rota | O que faz |
|---|---|---|
| `GET` | `/me` | Identidade da chave, limites e consumo |
| `GET` | `/quota` | Consumo da cota diária |
| `POST` | `/media/resolve` | Metadados e formatos de uma URL (não consome cota) |
| `POST` | `/downloads` | Cria um download → `202` |
| `GET` | `/downloads` | Lista, com filtros `status`, `platform`, `search` |
| `GET` | `/downloads/{id}` | Estado atual de um download |
| `POST` | `/downloads/{id}/cancel` | Cancela (idempotente) |
| `DELETE` | `/downloads/{id}` | Remove do histórico e apaga o arquivo |
| `GET` | `/downloads/{id}/download-url` | URL assinada, válida por 1 minuto |
| `GET` | `/downloads/{id}/file` | Os bytes, pela API |
| `GET` | `/webhooks` | Webhooks configurados (sem o segredo) |
| `POST` | `/webhooks/{id}/test` | Dispara um `ping` |
| `GET` | `/deliveries` | Histórico de notificações enviadas |

Paginação: `?page=1&perPage=20` (máximo 100). Respostas de lista trazem
`pagination` com `total`, `next_page` e `prev_page`.

O cancelamento é **idempotente**: cancelar algo que acabou de terminar sozinho
não é erro. A resposta devolve o status que de fato vigora — verifique o
`status`, em vez de assumir `CANCELED`.

---

## O painel (lado do operador)

**Administração → Integrações**, visível só para super admin.

### Criar uma integração

Nome, descrição e URL base do sistema (essa última é só referência, para
identificar de onde vem o tráfego), mais os limites:

- **Downloads por dia** — reseta à meia-noite de São Paulo. `0` = sem teto.
- **Downloads simultâneos** — impede gastar a cota do dia toda de uma vez. Cada
  download ocupa um slot do worker por minutos.
- **Chamadas por minuto** — por chave de API, não por integração: quando há
  várias chaves, o consumo descontrolado de uma não derruba as outras.
- **Tamanho máximo por arquivo** — `0` = sem teto.
- **IPs autorizados** — um por linha, IP ou CIDR. **Vazio aceita qualquer
  origem.**

Ao salvar, a chave de API aparece **uma única vez**. A janela não fecha sozinha
justamente por isso.

### As abas de uma integração

| Aba | Para quê |
|---|---|
| **Visão geral** | Downloads de hoje, em andamento, concluídos, armazenamento; chamadas e erros no período; situação das notificações |
| **Chaves de API** | Emitir, regerar, revogar; ver último uso e de qual IP |
| **Webhooks** | Cadastrar URL, escolher eventos, testar, rotacionar segredo, ativar/desativar |
| **Entregas** | O que foi enviado, o corpo exato, o que o endpoint respondeu, reenvio manual |
| **Requisições e IPs** | De onde vêm as chamadas (com aviso de IP fora da lista), erros agrupados por causa, e o log completo filtrável |
| **Downloads** | Os downloads daquela integração, com o motivo técnico das falhas |

A aba **Entregas** é a que encerra a discussão mais comum de qualquer integração
por webhook — "vocês enviaram?" / "não recebi" — com um registro que as duas
partes podem consultar.

### A conta de serviço

Cada integração possui uma conta de serviço (`integracao-<id>@sistemas.invalid`)
que é a dona dos downloads dela. Ela **não aparece na tela de Usuários** e
**não faz login**: autentica apenas por chave de API. Tentativas de editá-la,
redefinir senha ou removê-la pela tela de usuários são recusadas — a gestão é na
seção de integrações, que revoga as chaves junto ao remover.

---

## Como isto funciona por dentro

Útil para quem vai manter o código.

### Uma integração é uma conta

Cada integração possui uma linha em `users` com `kind = 'service'`, e os
downloads dela usam a coluna `downloads.user_id` de sempre.

Essa é a decisão que governa o resto: sem ela, cada recurso já existente — cota
diária, histórico paginado, expiração de arquivo, contabilidade de
armazenamento, eventos de tempo real, painel de downloads — precisaria de um
segundo caminho que fizesse a mesma coisa por um identificador diferente, e cada
correção futura teria de ser aplicada duas vezes.

O que a conta de serviço não compartilha é a forma de autenticar: ela não tem
senha utilizável (o hash guardado não é um bcrypt válido, então a comparação
falha para qualquer entrada) e é barrada no `/auth/login` e no middleware de
token.

### As rotas de download são as mesmas

`POST /v1/integration/downloads` e a criação da tela chamam a **mesma função**.
O middleware de chave de API deixa a conta de serviço no contexto pela mesma
chave que o login de usuário usa, e os handlers não sabem qual dos dois
autenticou. O que difere é apenas o formato da resposta.

### Os webhooks se penduram no stream que já existe

O worker já publica cada transição de status em um stream do Redis, que a API
consome para alimentar o SSE da tela. O despachante de webhooks se pendura no
**mesmo** consumidor, em vez de pedir que cada job avise os webhooks.

Isso mantém `internal/jobs` sem nenhuma noção de integração: qualquer transição
que apareça na tela do usuário chega ao sistema integrado pelo mesmo caminho, e
um estado novo no futuro não precisa ser publicado em dois lugares.

Um segundo consumidor só para webhooks criaria um grupo concorrente no mesmo
stream, e cada evento iria para apenas um dos dois — metade das atualizações
deixaria de aparecer na tela.

### A entrega roda no worker

Ela depende de um servidor de terceiro responder. Um endpoint lento na mão de um
cliente não pode ocupar a goroutine que atende as requisições HTTP de todos os
outros — e é justamente o cliente com problema que demora mais. A fila é
separada da de downloads pelo mesmo motivo.

O reenvio é o do asynq (backoff exponencial). Reimplementá-lo com uma coluna
`next_attempt_at` e um job varredor seria refazer, com menos garantias, o que a
fila já faz.

### Proteções da saída

A URL do webhook é escolhida por quem cadastra, e é o **servidor** que faz a
requisição. Sem cuidado, isso é um SSRF pronto. Três camadas:

1. **No cadastro**: só `https`, sem credencial na URL, e o host tem de resolver
   para endereço público — `169.254.169.254`, `10.0.0.0/8`, `127.0.0.1`,
   `100.64.0.0/10` e afins são recusados.
2. **Na discagem**: o dialer confere o IP real na hora de conectar, o que fecha
   a janela do DNS rebinding (validar o nome no cadastro e ele passar a resolver
   para um endereço interno depois).
3. **Sem redirecionamento**: um endpoint que responde `302` para um endereço
   interno não é seguido.

`WEBHOOK_ALLOW_PRIVATE=true` desliga tudo isso e existe **somente para
desenvolvimento**, onde o sistema integrado roda em localhost.

### A auditoria não entra no caminho da resposta

Cada chamada gera uma linha em `integration_requests`, gravada por escritores
dedicados que consomem uma fila em memória. Gravar de forma síncrona dobraria as
idas ao Postgres por requisição. O contrato é explícito: sob pressão extrema,
**perde-se registro, nunca disponibilidade**.

A poda roda de madrugada (`INTEGRATION_LOG_RETENTION_DAYS`, padrão 30 dias). As
entregas que **falharam** sobrevivem três vezes mais tempo — é nelas que alguém
vai procurar o motivo, muito depois do dia em que aconteceram.

### Configuração

| Variável | Para quê |
|---|---|
| `PUBLIC_API_URL` | URL pública da API, usada nos `links` do payload. Não é derivável da requisição: o evento é montado fora de qualquer requisição HTTP |
| `WEBHOOK_TIMEOUT` | Teto por tentativa de entrega (padrão 20s) |
| `WEBHOOK_PROGRESS_INTERVAL` | Espaçamento mínimo entre eventos de progresso do mesmo download (padrão 5s) |
| `WEBHOOK_ALLOW_PRIVATE` | **Só em desenvolvimento**: permite webhook para localhost |
| `INTEGRATION_LOG_RETENTION_DAYS` | Retenção da auditoria (padrão 30) |
| `DOCS_ENABLED` | Publica `/docs` e `/openapi.yaml` |

---

## Problemas comuns

**`invalid_api_key` com uma chave que eu sei que está certa.**
Confira se não há espaço ou quebra de linha coladas junto, e se o header é
`Authorization: Bearer <chave>` (com o espaço). A chave começa com `adk_live_` e
tem 52 caracteres. Se o proxy do seu ambiente reescreve `Authorization`, use
`X-API-Key`.

**`ip_not_allowed` e eu não sei qual é o meu IP.**
`GET /me` mostra a lista configurada. O IP que chegou até nós aparece no painel,
na aba **Requisições e IPs** — inclusive marcado como "fora da lista". É de lá
que se descobre o endereço real de saída do seu servidor.

**Recebo `quota_exceeded` mas ainda não baixei nada hoje.**
A cota conta pedidos **criados** no dia, no fuso de São Paulo — downloads
cancelados e falhados também contam, porque o trabalho foi gasto. `GET /quota`
mostra `daily_used` e `resets_at`.

**Meu webhook não recebe nada.**
Na ordem: (1) o webhook está **ativo** no painel? (2) o botão **Testar** entrega?
(3) a aba **Entregas** mostra tentativas e o que o seu endpoint respondeu. Se
mostra `401`, a verificação de assinatura do seu lado está recusando — o erro
quase sempre é verificar sobre o JSON reserializado em vez do corpo cru.

**O webhook foi desativado sozinho.**
Dez entregas seguidas abandonadas. O motivo está no painel, no próprio webhook.
Corrija o endpoint e reative pelo botão de liga/desliga — reativar zera o
contador de falhas.

**A URL de download expirou antes de eu usar.**
Ela vale um minuto, de propósito. Guarde o `id` do download e peça outra URL na
hora de baixar, em vez de guardar a URL.

**`download_not_ready` (409) logo depois de criar.**
Esperado: `POST /downloads` responde `202`, o arquivo ainda não existe. Espere o
webhook `download.completed` ou consulte `GET /downloads/{id}`.

**Recebi `download.progress` sem pedir.**
Alguém marcou "enviar também o progresso" no cadastro do webhook. Desmarque no
painel.
