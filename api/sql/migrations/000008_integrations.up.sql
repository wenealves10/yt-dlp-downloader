-- Integrações: o mesmo downloader, consumido por OUTRO SISTEMA em vez de por
-- uma pessoa na tela.
--
-- A decisão que governa todo este arquivo: uma integração É uma conta. Ela
-- possui uma linha própria em `users`, e os downloads dela usam a coluna
-- `downloads.user_id` de sempre. Sem isso, cada recurso já existente — cota
-- diária, histórico paginado, expiração de arquivo, contabilidade de
-- armazenamento, eventos de tempo real, painel de downloads — precisaria de um
-- segundo caminho que fizesse a mesma coisa por um identificador diferente, e
-- cada correção futura teria de ser aplicada duas vezes.
--
-- O que a conta de serviço NÃO compartilha com a conta de gente é a forma de
-- autenticar: ela não tem senha utilizável e nunca entra pelo /auth/login. Quem
-- prova identidade é a chave de API, e é por isso que `users.kind` existe.

-- ---------------------------------------------------------------------------
-- Conta de serviço
-- ---------------------------------------------------------------------------

CREATE TYPE "core"."user_kind" AS ENUM (
  'human',
  'service'
);

-- Sem este discriminador, as contas das integrações apareceriam misturadas na
-- listagem de usuários do painel, e — pior — aceitariam redefinição de senha e
-- login normal. Uma conta que só deveria ser alcançável por chave de API
-- ganharia um segundo caminho de entrada, que ninguém está observando.
ALTER TABLE "users" ADD COLUMN "kind" core.user_kind NOT NULL DEFAULT 'human';

CREATE INDEX ON "users" ("kind");

-- ---------------------------------------------------------------------------
-- Integrações
-- ---------------------------------------------------------------------------

CREATE TABLE "integrations" (
  "id" UUID PRIMARY KEY,

  -- A conta de serviço que possui os downloads desta integração. É o elo que
  -- permite reaproveitar todo o fluxo de download sem uma coluna nova em
  -- `downloads`.
  "user_id" UUID NOT NULL,

  "name" TEXT NOT NULL,
  "description" TEXT DEFAULT null,

  -- URL base do sistema integrado, guardada para referência do operador (é o
  -- "de onde vem" que aparece no painel). As notificações vão para a URL
  -- cadastrada em `integration_webhooks`, que é a que de fato recebe POST.
  "callback_base_url" TEXT DEFAULT null,

  -- Teto de downloads por dia, com o mesmo corte no fuso de São Paulo usado
  -- pelo contador do usuário comum.
  --
  -- O limite vive AQUI e não em `users.daily_limit` da conta de serviço, de
  -- propósito: duas colunas com o mesmo significado divergem no dia em que
  -- alguém editar uma só. O `daily_limit` da conta de serviço fica em zero e
  -- não é consultado por caminho nenhum.
  "daily_limit" INT NOT NULL DEFAULT 50,

  -- Teto de tamanho por arquivo. Zero desliga a checagem; os planos do usuário
  -- comum não se aplicam a um sistema.
  "max_file_size_bytes" BIGINT NOT NULL DEFAULT 0,

  -- Duas defesas diferentes contra o mesmo risco de indisponibilidade:
  -- `rate_limit_per_minute` limita a FREQUÊNCIA de chamadas (cada requisição
  -- custa pouco, mas um laço apertado derruba a API), e
  -- `max_concurrent_downloads` limita o TRABALHO simultâneo (cada download
  -- ocupa um slot do worker por minutos, e a cota diária sozinha permitiria
  -- gastar tudo de uma vez).
  "rate_limit_per_minute" INT NOT NULL DEFAULT 60,
  "max_concurrent_downloads" INT NOT NULL DEFAULT 3,

  -- Lista de IPs ou CIDRs autorizados. Vazia significa "qualquer origem": uma
  -- lista vazia que bloqueasse tudo faria toda integração nascer quebrada.
  "allowed_ips" TEXT[] NOT NULL DEFAULT '{}',

  "active" BOOLEAN NOT NULL DEFAULT true,
  "created_by" UUID DEFAULT null,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "deleted_at" TIMESTAMPTZ DEFAULT null
);

ALTER TABLE "integrations" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");
ALTER TABLE "integrations" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id");

-- Uma conta de serviço atende uma integração só. O vínculo é o que dá sentido a
-- "os downloads desta integração".
CREATE UNIQUE INDEX ON "integrations" ("user_id");

CREATE INDEX ON "integrations" ("active");
CREATE INDEX ON "integrations" ("deleted_at");
CREATE INDEX ON "integrations" (lower("name"));

-- ---------------------------------------------------------------------------
-- Chaves de API
-- ---------------------------------------------------------------------------

-- A chave é guardada como HASH, nunca em claro. O valor completo existe uma
-- única vez: na resposta que a criou. Um vazamento do banco não entrega acesso
-- às integrações, e nem quem opera o painel consegue recuperar a chave de um
-- cliente — só emitir outra.
--
-- SHA-256 e não bcrypt: a chave é gerada por nós com 256 bits de entropia, não
-- escolhida por uma pessoa. Não há dicionário para atacar, e um hash lento
-- entraria no caminho de CADA requisição autenticada.
CREATE TABLE "integration_api_keys" (
  "id" UUID PRIMARY KEY,
  "integration_id" UUID NOT NULL,
  "label" TEXT NOT NULL,

  -- Parte inicial legível ("adk_live_7f3a…"), para o painel e os logs
  -- identificarem QUAL chave sem nunca exibir a chave.
  "key_prefix" TEXT NOT NULL,
  "key_hash" TEXT NOT NULL,
  "last_four" TEXT NOT NULL,

  -- Regerar não apaga: revoga. O rastro de qual chave fez o quê continua
  -- legível depois da troca, que é justamente quando alguém vai querer olhar.
  "revoked_at" TIMESTAMPTZ DEFAULT null,
  "revoked_reason" TEXT DEFAULT null,
  "expires_at" TIMESTAMPTZ DEFAULT null,

  "last_used_at" TIMESTAMPTZ DEFAULT null,
  "last_used_ip" TEXT DEFAULT null,
  "created_by" UUID DEFAULT null,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE "integration_api_keys"
  ADD FOREIGN KEY ("integration_id") REFERENCES "integrations" ("id") ON DELETE CASCADE;
ALTER TABLE "integration_api_keys"
  ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id");

-- A autenticação é uma busca por este índice, e só por ele: o cliente manda a
-- chave, nós hasheamos e procuramos. Nunca varremos a tabela comparando.
CREATE UNIQUE INDEX ON "integration_api_keys" ("key_hash");

CREATE INDEX ON "integration_api_keys" ("integration_id");

-- ---------------------------------------------------------------------------
-- Webhooks
-- ---------------------------------------------------------------------------

CREATE TABLE "integration_webhooks" (
  "id" UUID PRIMARY KEY,
  "integration_id" UUID NOT NULL,
  "url" TEXT NOT NULL,

  -- Segredo de assinatura HMAC. Diferente da chave de API, este PRECISA ser
  -- legível: é o sistema do cliente que verifica a assinatura, e ele tem de
  -- poder copiar o mesmo valor que nós usamos para assinar.
  "secret" TEXT NOT NULL,

  -- Eventos assinados. Vazio significa "todos os de ciclo de vida" — progresso
  -- fica de fora porque são dezenas de POSTs por download.
  "events" TEXT[] NOT NULL DEFAULT '{}',
  "include_progress" BOOLEAN NOT NULL DEFAULT false,

  "active" BOOLEAN NOT NULL DEFAULT true,

  -- Diagnóstico de entrega. Sem isto, "o cliente diz que não recebeu" não tem
  -- resposta possível sem abrir log de container.
  "last_delivery_at" TIMESTAMPTZ DEFAULT null,
  "last_status_code" INT DEFAULT null,
  "last_error" TEXT DEFAULT null,
  "consecutive_failures" INT NOT NULL DEFAULT 0,
  "disabled_reason" TEXT DEFAULT null,

  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "deleted_at" TIMESTAMPTZ DEFAULT null
);

ALTER TABLE "integration_webhooks"
  ADD FOREIGN KEY ("integration_id") REFERENCES "integrations" ("id") ON DELETE CASCADE;

CREATE INDEX ON "integration_webhooks" ("integration_id");
CREATE INDEX ON "integration_webhooks" ("active");

CREATE TYPE "core"."webhook_delivery_status" AS ENUM (
  'PENDING',
  'DELIVERED',
  'FAILED'
);

-- Cada tentativa de notificação, com a resposta que o destino deu.
--
-- O payload fica gravado junto porque reentrega tem de mandar EXATAMENTE o que
-- foi prometido. Remontá-lo a partir do estado atual do download entregaria
-- "concluído" para um evento que na hora dizia "em andamento", e o cliente que
-- processa a fila em ordem veria uma história que não aconteceu.
CREATE TABLE "integration_webhook_deliveries" (
  "id" UUID PRIMARY KEY,
  "webhook_id" UUID NOT NULL,
  "integration_id" UUID NOT NULL,
  "event_type" TEXT NOT NULL,
  "download_id" UUID DEFAULT null,
  "payload" JSONB NOT NULL,
  "status" core.webhook_delivery_status NOT NULL DEFAULT 'PENDING',
  "attempts" INT NOT NULL DEFAULT 0,
  "last_status_code" INT DEFAULT null,
  "last_error" TEXT DEFAULT null,
  "duration_ms" INT NOT NULL DEFAULT 0,
  "delivered_at" TIMESTAMPTZ DEFAULT null,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE "integration_webhook_deliveries"
  ADD FOREIGN KEY ("webhook_id") REFERENCES "integration_webhooks" ("id") ON DELETE CASCADE;
ALTER TABLE "integration_webhook_deliveries"
  ADD FOREIGN KEY ("integration_id") REFERENCES "integrations" ("id") ON DELETE CASCADE;

CREATE INDEX ON "integration_webhook_deliveries" ("integration_id", "created_at" DESC);
CREATE INDEX ON "integration_webhook_deliveries" ("webhook_id", "created_at" DESC);
CREATE INDEX ON "integration_webhook_deliveries" ("status");
CREATE INDEX ON "integration_webhook_deliveries" ("download_id");

-- ---------------------------------------------------------------------------
-- Auditoria de requisições
-- ---------------------------------------------------------------------------

-- Uma linha por chamada autenticada da API de integrações. É o que responde,
-- no painel, as perguntas que aparecem quando algo vai mal: de qual IP veio,
-- qual chave usou, qual rota, o que respondemos e quanto demorou.
--
-- O IP é TEXT e não INET porque tudo que fazemos com ele é exibir, filtrar e
-- agrupar. INET obrigaria o código gerado a trabalhar com prefixos de rede
-- para guardar um endereço único.
CREATE TABLE "integration_requests" (
  "id" BIGSERIAL PRIMARY KEY,
  "integration_id" UUID NOT NULL,
  "api_key_id" UUID DEFAULT null,
  "method" TEXT NOT NULL,
  "path" TEXT NOT NULL,
  "status_code" INT NOT NULL,
  "ip" TEXT NOT NULL DEFAULT '',
  "user_agent" TEXT NOT NULL DEFAULT '',
  "duration_ms" INT NOT NULL DEFAULT 0,

  -- Código de erro estável (o mesmo que vai no corpo da resposta), para o
  -- painel agrupar falhas por causa em vez de por texto.
  "error_code" TEXT DEFAULT null,
  "download_id" UUID DEFAULT null,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE "integration_requests"
  ADD FOREIGN KEY ("integration_id") REFERENCES "integrations" ("id") ON DELETE CASCADE;

CREATE INDEX ON "integration_requests" ("integration_id", "created_at" DESC);
CREATE INDEX ON "integration_requests" ("ip");
CREATE INDEX ON "integration_requests" ("status_code");
CREATE INDEX ON "integration_requests" ("created_at");
