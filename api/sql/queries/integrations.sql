-- Consultas da seção de integrações: as contas de SISTEMA que consomem a
-- plataforma por API, as chaves que as autenticam, os webhooks que avisam o
-- sistema integrado e a auditoria de tudo que chegou por essa porta.

-- ---------------------------------------------------------------------------
-- Conta de serviço
-- ---------------------------------------------------------------------------

-- A conta de serviço nasce sem senha utilizável: `hashed_password` recebe um
-- valor que nenhum bcrypt aceita como válido, então nem uma senha em branco
-- autentica. Quem prova identidade aqui é a chave de API.
--
-- `daily_limit` fica em zero porque a cota da integração vive na linha de
-- `integrations`; deixar um número aqui criaria dois limites com o mesmo nome.
-- name: CreateServiceUser :one
INSERT INTO users (
  id, full_name, email, hashed_password, plan, daily_limit,
  active, is_verified, kind, password_changed_at
)
VALUES ($1, $2, $3, $4, 'enterprise', 0, true, true, 'service', now())
RETURNING *;

-- ---------------------------------------------------------------------------
-- Integrações
-- ---------------------------------------------------------------------------

-- name: CreateIntegration :one
INSERT INTO integrations (
  id, user_id, name, description, callback_base_url, daily_limit,
  max_file_size_bytes, rate_limit_per_minute, max_concurrent_downloads,
  allowed_ips, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetIntegrationByID :one
SELECT * FROM integrations
WHERE id = $1
  AND deleted_at IS NULL;

-- Usada pelo despachante de webhooks: o evento de download carrega o
-- `user_id`, e é por ele que se descobre se aquele download pertence a uma
-- integração.
-- name: GetIntegrationByUserID :one
SELECT * FROM integrations
WHERE user_id = $1
  AND deleted_at IS NULL;

-- Listagem do painel. Os números vêm na mesma consulta porque a tela mostra
-- todos eles juntos, e uma chamada por integração transformaria a abertura da
-- página em N+1 idas ao banco.
--
-- `downloads_today` usa o MESMO corte de dia do contador do usuário comum
-- (meia-noite em São Paulo, convertida para UTC). Um corte em UTC faria a cota
-- virar às 21h e o painel discordaria da API.
-- name: ListIntegrations :many
SELECT
  i.*,
  u.email AS service_email,
  COUNT(d.id) FILTER (WHERE d.deleted_at IS NULL)::bigint AS downloads_total,
  COUNT(d.id) FILTER (
    WHERE d.deleted_at IS NULL
      AND d.created_at >= TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE))
      AND d.created_at <  TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE + INTERVAL '1 day'))
  )::bigint AS downloads_today,
  COUNT(d.id) FILTER (
    WHERE d.deleted_at IS NULL AND d.status IN ('PENDING', 'PROCESSING', 'RETRYING')
  )::bigint AS downloads_active,
  COALESCE(SUM(d.file_size_bytes) FILTER (
    WHERE d.deleted_at IS NULL AND d.status = 'COMPLETED'
  ), 0)::bigint AS storage_bytes,
  (
    SELECT COUNT(*) FROM integration_api_keys k
    WHERE k.integration_id = i.id AND k.revoked_at IS NULL
  )::bigint AS active_keys,
  -- O cast explícito é necessário: sem ele o sqlc não consegue inferir o tipo
  -- da subconsulta e gera interface{}, que a camada de cima não sabe formatar.
  (
    SELECT MAX(k.last_used_at) FROM integration_api_keys k
    WHERE k.integration_id = i.id
  )::timestamptz AS last_used_at
FROM integrations i
JOIN users u ON u.id = i.user_id
LEFT JOIN downloads d ON d.user_id = i.user_id
WHERE i.deleted_at IS NULL
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(i.name) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(COALESCE(i.description, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  )
  AND (sqlc.narg('active')::boolean IS NULL OR i.active = sqlc.narg('active')::boolean)
GROUP BY i.id, u.email
ORDER BY i.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountIntegrations :one
SELECT COUNT(*) FROM integrations i
WHERE i.deleted_at IS NULL
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(i.name) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(COALESCE(i.description, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  )
  AND (sqlc.narg('active')::boolean IS NULL OR i.active = sqlc.narg('active')::boolean);

-- name: UpdateIntegration :one
UPDATE integrations
SET
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description),
  callback_base_url = COALESCE(sqlc.narg('callback_base_url'), callback_base_url),
  daily_limit = COALESCE(sqlc.narg('daily_limit'), daily_limit),
  max_file_size_bytes = COALESCE(sqlc.narg('max_file_size_bytes'), max_file_size_bytes),
  rate_limit_per_minute = COALESCE(sqlc.narg('rate_limit_per_minute'), rate_limit_per_minute),
  max_concurrent_downloads = COALESCE(sqlc.narg('max_concurrent_downloads'), max_concurrent_downloads),
  -- O array não usa COALESCE com o próprio valor: uma lista VAZIA é uma
  -- escolha legítima ("aceitar qualquer IP"), e COALESCE não sabe distinguir
  -- "não mandei" de "mandei vazio". O sentinela é o NULL do narg.
  allowed_ips = CASE
      WHEN sqlc.narg('allowed_ips')::text[] IS NULL THEN allowed_ips
      ELSE sqlc.narg('allowed_ips')::text[]
    END,
  active = COALESCE(sqlc.narg('active'), active),
  updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- Remoção lógica, como a de usuário: o histórico de downloads referencia a
-- conta de serviço e responde pela contabilidade de armazenamento.
-- name: SoftDeleteIntegration :exec
UPDATE integrations
SET deleted_at = now(), active = false, updated_at = now()
WHERE id = $1;

-- Cota do dia. Mesmo corte de fuso do contador do usuário comum.
-- name: CountIntegrationDownloadsToday :one
SELECT COUNT(*) FROM downloads
WHERE user_id = $1
  AND deleted_at IS NULL
  AND created_at >= TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE))
  AND created_at <  TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE + INTERVAL '1 day'));

-- Trabalho em voo. A cota diária sozinha permitiria disparar tudo de uma vez e
-- ocupar o worker inteiro; este número é o que sustenta o limite de simultâneos.
-- name: CountIntegrationActiveDownloads :one
SELECT COUNT(*) FROM downloads
WHERE user_id = $1
  AND deleted_at IS NULL
  AND status IN ('PENDING', 'PROCESSING', 'RETRYING');

-- Resumo da integração para a aba de visão geral.
-- name: IntegrationDownloadSummary :one
SELECT
  COUNT(*) FILTER (WHERE deleted_at IS NULL)::bigint AS total,
  COUNT(*) FILTER (
    WHERE deleted_at IS NULL
      AND created_at >= TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE))
      AND created_at <  TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE + INTERVAL '1 day'))
  )::bigint AS today,
  COUNT(*) FILTER (
    WHERE deleted_at IS NULL AND status IN ('PENDING', 'PROCESSING', 'RETRYING')
  )::bigint AS active,
  COUNT(*) FILTER (WHERE deleted_at IS NULL AND status = 'COMPLETED')::bigint AS completed,
  COUNT(*) FILTER (WHERE deleted_at IS NULL AND status = 'FAILED')::bigint AS failed,
  COUNT(*) FILTER (WHERE deleted_at IS NULL AND status = 'CANCELED')::bigint AS canceled,
  COALESCE(SUM(file_size_bytes) FILTER (
    WHERE deleted_at IS NULL AND status = 'COMPLETED'
  ), 0)::bigint AS storage_bytes,
  COALESCE(SUM(file_size_bytes) FILTER (WHERE deleted_at IS NULL), 0)::bigint AS transferred_bytes
FROM downloads
WHERE user_id = $1;

-- ---------------------------------------------------------------------------
-- Chaves de API
-- ---------------------------------------------------------------------------

-- name: CreateIntegrationAPIKey :one
INSERT INTO integration_api_keys (
  id, integration_id, label, key_prefix, key_hash, last_four, expires_at, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- O caminho de autenticação, e é por isso que traz tudo de uma vez: a chave, a
-- integração e a conta de serviço. Três consultas aqui seriam três idas ao
-- banco em CADA requisição da API de integrações.
--
-- A busca é pelo hash, que tem índice único. A chave em claro nunca é
-- comparada contra nada no banco.
-- name: GetIntegrationKeyByHash :one
SELECT
  k.id                AS key_id,
  k.label             AS key_label,
  k.key_prefix        AS key_prefix,
  k.revoked_at        AS key_revoked_at,
  k.expires_at        AS key_expires_at,
  sqlc.embed(i),
  sqlc.embed(u)
FROM integration_api_keys k
JOIN integrations i ON i.id = k.integration_id
JOIN users u ON u.id = i.user_id
WHERE k.key_hash = $1
LIMIT 1;

-- name: ListIntegrationAPIKeys :many
SELECT * FROM integration_api_keys
WHERE integration_id = $1
ORDER BY revoked_at IS NOT NULL, created_at DESC;

-- name: GetIntegrationAPIKey :one
SELECT * FROM integration_api_keys
WHERE id = $1;

-- Revogar é idempotente e preserva o carimbo original: uma segunda revogação
-- não deve reescrever a data em que a chave de fato saiu do ar.
-- name: RevokeIntegrationAPIKey :one
UPDATE integration_api_keys
SET revoked_at = COALESCE(revoked_at, now()),
    revoked_reason = COALESCE(revoked_reason, sqlc.narg('reason'))
WHERE id = $1
RETURNING *;

-- Usada ao regerar: a chave nova entra e todas as anteriores caem na mesma
-- operação, porque "regerar" significa que o valor antigo não vale mais.
-- name: RevokeOtherIntegrationAPIKeys :exec
UPDATE integration_api_keys
SET revoked_at = now(),
    revoked_reason = COALESCE(revoked_reason, sqlc.narg('reason'))
WHERE integration_id = $1
  AND id <> sqlc.arg('keep_id')
  AND revoked_at IS NULL;

-- Gravado a cada uso. É o que responde "esta chave ainda é usada?" antes de
-- revogar uma que parece esquecida.
-- name: TouchIntegrationAPIKey :exec
UPDATE integration_api_keys
SET last_used_at = now(), last_used_ip = $2
WHERE id = $1;

-- ---------------------------------------------------------------------------
-- Webhooks
-- ---------------------------------------------------------------------------

-- name: CreateIntegrationWebhook :one
INSERT INTO integration_webhooks (
  id, integration_id, url, secret, events, include_progress
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListIntegrationWebhooks :many
SELECT * FROM integration_webhooks
WHERE integration_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- Consultada pelo despachante a cada evento de ciclo de vida. Só os ativos: um
-- webhook desligado não deve nem gerar linha de entrega.
-- name: ListActiveIntegrationWebhooks :many
SELECT * FROM integration_webhooks
WHERE integration_id = $1
  AND deleted_at IS NULL
  AND active = true
ORDER BY created_at;

-- name: GetIntegrationWebhook :one
SELECT * FROM integration_webhooks
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UpdateIntegrationWebhook :one
UPDATE integration_webhooks
SET
  url = COALESCE(sqlc.narg('url'), url),
  events = CASE
      WHEN sqlc.narg('events')::text[] IS NULL THEN events
      ELSE sqlc.narg('events')::text[]
    END,
  include_progress = COALESCE(sqlc.narg('include_progress'), include_progress),
  active = COALESCE(sqlc.narg('active'), active),
  secret = COALESCE(sqlc.narg('secret'), secret),
  -- Reativar limpa o histórico de falhas. Sem isso, um webhook religado depois
  -- de o cliente consertar o endpoint já nasceria a uma falha do desligamento
  -- automático.
  consecutive_failures = CASE
      WHEN COALESCE(sqlc.narg('active'), active) THEN 0
      ELSE consecutive_failures
    END,
  disabled_reason = CASE
      WHEN COALESCE(sqlc.narg('active'), active) THEN NULL
      ELSE disabled_reason
    END,
  updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteIntegrationWebhook :exec
UPDATE integration_webhooks
SET deleted_at = now(), active = false, updated_at = now()
WHERE id = $1;

-- name: MarkWebhookDelivered :exec
UPDATE integration_webhooks
SET last_delivery_at = now(),
    last_status_code = $2,
    last_error = NULL,
    consecutive_failures = 0,
    updated_at = now()
WHERE id = $1;

-- O contador de falhas consecutivas é o que permite desligar sozinho um
-- endpoint que morreu: sem ele, um destino que responde 500 para sempre
-- receberia reentrega eternamente.
-- name: MarkWebhookFailed :one
UPDATE integration_webhooks
SET last_delivery_at = now(),
    last_status_code = sqlc.narg('status_code'),
    last_error = sqlc.narg('error'),
    consecutive_failures = consecutive_failures + 1,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DisableIntegrationWebhook :exec
UPDATE integration_webhooks
SET active = false, disabled_reason = $2, updated_at = now()
WHERE id = $1;

-- ---------------------------------------------------------------------------
-- Entregas
-- ---------------------------------------------------------------------------

-- name: CreateWebhookDelivery :one
INSERT INTO integration_webhook_deliveries (
  id, webhook_id, integration_id, event_type, download_id, payload
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetWebhookDelivery :one
SELECT * FROM integration_webhook_deliveries
WHERE id = $1;

-- name: ListWebhookDeliveries :many
SELECT
  d.*,
  w.url AS webhook_url
FROM integration_webhook_deliveries d
JOIN integration_webhooks w ON w.id = d.webhook_id
WHERE d.integration_id = $1
  AND (sqlc.narg('status')::core.webhook_delivery_status IS NULL
       OR d.status = sqlc.narg('status')::core.webhook_delivery_status)
  AND (sqlc.narg('event_type')::text IS NULL OR d.event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('download_id')::uuid IS NULL OR d.download_id = sqlc.narg('download_id')::uuid)
ORDER BY d.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWebhookDeliveries :one
SELECT COUNT(*) FROM integration_webhook_deliveries d
WHERE d.integration_id = $1
  AND (sqlc.narg('status')::core.webhook_delivery_status IS NULL
       OR d.status = sqlc.narg('status')::core.webhook_delivery_status)
  AND (sqlc.narg('event_type')::text IS NULL OR d.event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('download_id')::uuid IS NULL OR d.download_id = sqlc.narg('download_id')::uuid);

-- O cast explícito não é decoração: o mesmo parâmetro é usado como valor da
-- coluna e dentro do CASE, e sem ele o Postgres deduz tipos diferentes para os
-- dois usos e recusa a consulta com "inconsistent types deduced for parameter".
-- O sintoma é cruel: a entrega acontece, o cliente recebe, e o registro dela
-- nunca é atualizado — o painel mostra tudo como pendente para sempre.
-- name: UpdateWebhookDeliveryResult :exec
UPDATE integration_webhook_deliveries
SET status = sqlc.arg('status')::core.webhook_delivery_status,
    attempts = attempts + 1,
    last_status_code = sqlc.narg('status_code'),
    last_error = sqlc.narg('error'),
    duration_ms = sqlc.arg('duration_ms'),
    delivered_at = CASE
        WHEN sqlc.arg('status')::core.webhook_delivery_status = 'DELIVERED' THEN now()
        ELSE delivered_at
      END
WHERE id = sqlc.arg('id');

-- Números da aba de entregas, na janela que a tela pediu.
-- name: WebhookDeliveryStats :one
SELECT
  COUNT(*)::bigint AS total,
  COUNT(*) FILTER (WHERE status = 'DELIVERED')::bigint AS delivered,
  COUNT(*) FILTER (WHERE status = 'FAILED')::bigint AS failed,
  COUNT(*) FILTER (WHERE status = 'PENDING')::bigint AS pending,
  COALESCE(AVG(duration_ms) FILTER (WHERE status = 'DELIVERED'), 0)::int AS avg_duration_ms
FROM integration_webhook_deliveries
WHERE integration_id = $1
  AND created_at >= now() - (sqlc.arg('window_hours')::int * INTERVAL '1 hour');

-- ---------------------------------------------------------------------------
-- Auditoria de requisições
-- ---------------------------------------------------------------------------

-- name: CreateIntegrationRequest :exec
INSERT INTO integration_requests (
  integration_id, api_key_id, method, path, status_code,
  ip, user_agent, duration_ms, error_code, download_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- A listagem de auditoria. `status_class` filtra por faixa (2, 4, 5) em vez de
-- por código exato: quem investiga quer "os erros do cliente" ou "os nossos",
-- não especificamente um 429.
-- name: ListIntegrationRequests :many
SELECT * FROM integration_requests
WHERE integration_id = $1
  AND (sqlc.narg('ip')::text IS NULL OR ip = sqlc.narg('ip')::text)
  AND (sqlc.narg('status_class')::int IS NULL
       OR status_code / 100 = sqlc.narg('status_class')::int)
  AND (sqlc.narg('path')::text IS NULL OR path LIKE '%' || sqlc.narg('path')::text || '%')
  AND (sqlc.narg('error_code')::text IS NULL OR error_code = sqlc.narg('error_code')::text)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountIntegrationRequests :one
SELECT COUNT(*) FROM integration_requests
WHERE integration_id = $1
  AND (sqlc.narg('ip')::text IS NULL OR ip = sqlc.narg('ip')::text)
  AND (sqlc.narg('status_class')::int IS NULL
       OR status_code / 100 = sqlc.narg('status_class')::int)
  AND (sqlc.narg('path')::text IS NULL OR path LIKE '%' || sqlc.narg('path')::text || '%')
  AND (sqlc.narg('error_code')::text IS NULL OR error_code = sqlc.narg('error_code')::text);

-- name: IntegrationRequestStats :one
SELECT
  COUNT(*)::bigint AS total,
  COUNT(*) FILTER (WHERE status_code < 400)::bigint AS ok,
  COUNT(*) FILTER (WHERE status_code BETWEEN 400 AND 499)::bigint AS client_errors,
  COUNT(*) FILTER (WHERE status_code >= 500)::bigint AS server_errors,
  COUNT(*) FILTER (WHERE status_code = 429)::bigint AS throttled,
  COUNT(DISTINCT ip)::bigint AS unique_ips,
  COALESCE(AVG(duration_ms), 0)::int AS avg_duration_ms,
  COALESCE(MAX(duration_ms), 0)::int AS max_duration_ms
FROM integration_requests
WHERE integration_id = $1
  AND created_at >= now() - (sqlc.arg('window_hours')::int * INTERVAL '1 hour');

-- De onde as chamadas vêm. É a resposta para "quem está batendo na minha API",
-- que é exatamente o que se quer saber quando o volume sobe sem explicação.
-- name: IntegrationTopIPs :many
SELECT
  ip,
  COUNT(*)::bigint AS requests,
  COUNT(*) FILTER (WHERE status_code >= 400)::bigint AS errors,
  MAX(created_at) AS last_seen
FROM integration_requests
WHERE integration_id = $1
  AND created_at >= now() - (sqlc.arg('window_hours')::int * INTERVAL '1 hour')
  AND ip <> ''
GROUP BY ip
ORDER BY requests DESC
LIMIT $2;

-- Falhas agrupadas por CAUSA, não por texto. É por isso que `error_code` é
-- gravado junto da resposta: o painel mostra "quota_exceeded: 42" em vez de
-- quarenta e duas linhas iguais.
-- name: IntegrationTopErrors :many
SELECT
  COALESCE(error_code, 'sem_codigo') AS error_code,
  status_code,
  COUNT(*)::bigint AS occurrences,
  MAX(created_at) AS last_seen
FROM integration_requests
WHERE integration_id = $1
  AND status_code >= 400
  AND created_at >= now() - (sqlc.arg('window_hours')::int * INTERVAL '1 hour')
GROUP BY error_code, status_code
ORDER BY occurrences DESC
LIMIT $2;

-- ---------------------------------------------------------------------------
-- Retenção
-- ---------------------------------------------------------------------------

-- As duas tabelas de auditoria crescem a cada chamada e a cada notificação.
-- Sem poda, a de requisições passa o histórico de downloads em volume e o
-- painel fica lento por dado que ninguém mais vai olhar.
-- name: PruneIntegrationRequests :execrows
DELETE FROM integration_requests
WHERE created_at < now() - (sqlc.arg('keep_days')::int * INTERVAL '1 day');

-- Entregas concluídas são podadas; as que falharam sobrevivem mais tempo,
-- porque é justamente nelas que alguém vai procurar o motivo.
-- name: PruneWebhookDeliveries :execrows
DELETE FROM integration_webhook_deliveries
WHERE (status = 'DELIVERED' AND created_at < now() - (sqlc.arg('keep_days')::int * INTERVAL '1 day'))
   OR (status <> 'DELIVERED' AND created_at < now() - (sqlc.arg('keep_days')::int * 3 * INTERVAL '1 day'));

-- ---------------------------------------------------------------------------
-- Downloads da integração
-- ---------------------------------------------------------------------------

-- A listagem que a API de integrações e a aba de downloads do painel usam.
--
-- Existe separada de GetDownloadsByUser porque um sistema integrado precisa
-- filtrar: o caso normal dele é "me devolva o que ainda está em andamento" ou
-- "o que falhou hoje", e sem filtro no servidor ele teria de paginar o
-- histórico inteiro para descobrir isso.
-- name: ListIntegrationDownloads :many
SELECT * FROM downloads
WHERE user_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::core.download_status IS NULL
       OR status = sqlc.narg('status')::core.download_status)
  AND (sqlc.narg('platform')::text IS NULL OR platform = sqlc.narg('platform')::text)
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(title) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(original_url) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  )
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountIntegrationDownloads :one
SELECT COUNT(*) FROM downloads
WHERE user_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::core.download_status IS NULL
       OR status = sqlc.narg('status')::core.download_status)
  AND (sqlc.narg('platform')::text IS NULL OR platform = sqlc.narg('platform')::text)
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(title) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(original_url) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  );
