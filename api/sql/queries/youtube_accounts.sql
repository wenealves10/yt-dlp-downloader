-- name: CreateYoutubeAccount :one
INSERT INTO youtube_accounts (
  id, label, email, status, profile_dir, priority, created_by
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetYoutubeAccounts :many
SELECT * FROM youtube_accounts
WHERE deleted_at IS NULL
ORDER BY priority ASC, created_at ASC;

-- name: GetYoutubeAccountByID :one
SELECT * FROM youtube_accounts
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CountYoutubeAccounts :one
SELECT COUNT(*) FROM youtube_accounts
WHERE deleted_at IS NULL;

-- Reivindica a conta autenticada usada há mais tempo e já registra o uso na
-- mesma operação, de modo que o rodízio não dependa de um UPDATE posterior.
--
-- FOR UPDATE SKIP LOCKED faz dois workers simultâneos pegarem contas
-- diferentes em vez de disputarem a mesma linha. O parâmetro exclude carrega
-- as contas já descartadas nesta tentativa, o que permite passar para a
-- próxima quando uma sessão se revela inválida.
--
-- Não é rotação para contornar limites do YouTube: é distribuição justa entre
-- as contas legítimas do próprio administrador, respeitando a prioridade.
-- name: ClaimYoutubeAccount :one
UPDATE youtube_accounts
SET last_used_at = now(), updated_at = now()
WHERE id = (
  SELECT id FROM youtube_accounts
  WHERE deleted_at IS NULL
    AND active = true
    AND status = 'AUTHENTICATED'
    AND NOT (id = ANY(sqlc.arg('exclude')::uuid[]))
  ORDER BY priority ASC, last_used_at ASC NULLS FIRST, created_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CountAuthenticatedYoutubeAccounts :one
SELECT COUNT(*) FROM youtube_accounts
WHERE deleted_at IS NULL
  AND active = true
  AND status = 'AUTHENTICATED';

-- name: GetYoutubeAccountsForHealthCheck :many
SELECT * FROM youtube_accounts
WHERE deleted_at IS NULL
  AND active = true
  AND status IN ('AUTHENTICATED', 'REQUIRES_AUTH', 'AWAITING_LOGIN', 'ERROR')
ORDER BY last_checked_at ASC NULLS FIRST;

-- name: UpdateYoutubeAccount :one
UPDATE youtube_accounts
SET
  label = COALESCE(sqlc.narg('label'), label),
  email = COALESCE(sqlc.narg('email'), email),
  status = COALESCE(sqlc.narg('status'), status),
  active = COALESCE(sqlc.narg('active'), active),
  priority = COALESCE(sqlc.narg('priority'), priority),
  updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- O cast explícito é necessário: sem ele o Postgres deduz tipos diferentes para
-- o mesmo parâmetro, usado como valor da coluna e dentro do CASE.
-- name: UpdateYoutubeAccountStatus :one
UPDATE youtube_accounts
SET
  status = sqlc.arg('status'),
  last_error = sqlc.narg('last_error'),
  last_checked_at = now(),
  last_authenticated_at = CASE
    WHEN sqlc.arg('status')::core.youtube_account_status = 'AUTHENTICATED' THEN now()
    ELSE last_authenticated_at
  END,
  updated_at = now()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL
RETURNING *;


-- name: DeleteYoutubeAccount :exec
UPDATE youtube_accounts
SET deleted_at = now(), active = false, status = 'DISABLED', updated_at = now()
WHERE id = $1;
