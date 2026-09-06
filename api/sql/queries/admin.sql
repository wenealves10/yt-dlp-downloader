-- Consultas exclusivas do painel de Super Admin.
--
-- Todas ignoram registros removidos e, quando fazem contas de armazenamento,
-- somam apenas o que ainda existe no bucket.

-- ---------------------------------------------------------------------------
-- Usuários
-- ---------------------------------------------------------------------------

-- Listagem paginada com busca por nome/e-mail e filtros opcionais. Os
-- sqlc.narg vazios desativam o próprio filtro, o que evita uma consulta por
-- combinação de filtros.
-- name: AdminListUsers :many
SELECT
  u.*,
  COUNT(d.id) FILTER (WHERE d.deleted_at IS NULL)::bigint AS downloads_total,
  COALESCE(SUM(d.file_size_bytes) FILTER (
    WHERE d.deleted_at IS NULL AND d.status = 'COMPLETED'
  ), 0)::bigint AS storage_bytes
FROM users u
LEFT JOIN downloads d ON d.user_id = u.id
WHERE u.deleted_at IS NULL
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(u.full_name) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(u.email) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  )
  AND (sqlc.narg('plan')::core.plan_type IS NULL OR u.plan = sqlc.narg('plan')::core.plan_type)
  AND (sqlc.narg('role')::core.user_role IS NULL OR u.role = sqlc.narg('role')::core.user_role)
  AND (sqlc.narg('active')::boolean IS NULL OR u.active = sqlc.narg('active')::boolean)
GROUP BY u.id
ORDER BY u.created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountUsers :one
SELECT COUNT(*) FROM users u
WHERE u.deleted_at IS NULL
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(u.full_name) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(u.email) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  )
  AND (sqlc.narg('plan')::core.plan_type IS NULL OR u.plan = sqlc.narg('plan')::core.plan_type)
  AND (sqlc.narg('role')::core.user_role IS NULL OR u.role = sqlc.narg('role')::core.user_role)
  AND (sqlc.narg('active')::boolean IS NULL OR u.active = sqlc.narg('active')::boolean);

-- name: AdminGetUserDetail :one
SELECT
  u.*,
  COUNT(d.id) FILTER (WHERE d.deleted_at IS NULL)::bigint AS downloads_total,
  COUNT(d.id) FILTER (WHERE d.deleted_at IS NULL AND d.status = 'COMPLETED')::bigint AS downloads_completed,
  COUNT(d.id) FILTER (WHERE d.deleted_at IS NULL AND d.status = 'FAILED')::bigint AS downloads_failed,
  COALESCE(SUM(d.file_size_bytes) FILTER (
    WHERE d.deleted_at IS NULL AND d.status = 'COMPLETED'
  ), 0)::bigint AS storage_bytes,
  COALESCE(SUM(d.file_size_bytes) FILTER (WHERE d.deleted_at IS NULL), 0)::bigint AS transferred_bytes
FROM users u
LEFT JOIN downloads d ON d.user_id = u.id
WHERE u.id = $1
  AND u.deleted_at IS NULL
GROUP BY u.id;

-- Atualização vinda do painel. Diferente de UpdateUser, esta pode mexer no
-- papel — e é por isso que ela é separada: a rota de perfil do usuário comum
-- nunca deve conseguir se promover.
-- name: AdminUpdateUser :one
UPDATE users
SET
  full_name = COALESCE(sqlc.narg('full_name'), full_name),
  email = COALESCE(sqlc.narg('email'), email),
  plan = COALESCE(sqlc.narg('plan'), plan),
  role = COALESCE(sqlc.narg('role'), role),
  daily_limit = COALESCE(sqlc.narg('daily_limit'), daily_limit),
  active = COALESCE(sqlc.narg('active'), active),
  is_verified = COALESCE(sqlc.narg('is_verified'), is_verified),
  hashed_password = COALESCE(sqlc.narg('hashed_password'), hashed_password),
  password_changed_at = COALESCE(sqlc.narg('password_changed_at'), password_changed_at),
  updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- Remoção lógica. O histórico de downloads é preservado de propósito: ele
-- referencia users(id) e ainda responde pela contabilidade de armazenamento.
-- name: AdminSoftDeleteUser :exec
UPDATE users
SET deleted_at = now(), active = false, updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: AdminCountSuperAdmins :one
SELECT COUNT(*) FROM users
WHERE role = 'super_admin'
  AND active = true
  AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Downloads
-- ---------------------------------------------------------------------------

-- name: AdminListDownloads :many
SELECT
  d.*,
  u.full_name AS user_name,
  u.email AS user_email
FROM downloads d
JOIN users u ON u.id = d.user_id
WHERE d.deleted_at IS NULL
  AND (sqlc.narg('user_id')::uuid IS NULL OR d.user_id = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('status')::core.download_status IS NULL OR d.status = sqlc.narg('status')::core.download_status)
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(d.title) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(u.email) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  )
ORDER BY d.created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountDownloads :one
SELECT COUNT(*) FROM downloads d
JOIN users u ON u.id = d.user_id
WHERE d.deleted_at IS NULL
  AND (sqlc.narg('user_id')::uuid IS NULL OR d.user_id = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('status')::core.download_status IS NULL OR d.status = sqlc.narg('status')::core.download_status)
  AND (
    sqlc.narg('search')::text IS NULL
    OR lower(d.title) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
    OR lower(u.email) LIKE '%' || lower(sqlc.narg('search')::text) || '%'
  );


-- ---------------------------------------------------------------------------
-- Métricas
-- ---------------------------------------------------------------------------

-- Números do período escolhido. Um único SELECT em vez de seis: o painel abre
-- com todos eles na tela ao mesmo tempo.
-- name: AdminDownloadsSummary :one
SELECT
  COUNT(*)::bigint AS total,
  COUNT(*) FILTER (WHERE status = 'COMPLETED')::bigint AS completed,
  COUNT(*) FILTER (WHERE status = 'FAILED')::bigint AS failed,
  COUNT(*) FILTER (WHERE status IN ('PENDING', 'PROCESSING', 'RETRYING'))::bigint AS processing,
  COUNT(*) FILTER (WHERE status = 'EXPIRED')::bigint AS expired,
  COUNT(DISTINCT user_id)::bigint AS active_users,
  COALESCE(SUM(file_size_bytes), 0)::bigint AS transferred_bytes
FROM downloads
WHERE deleted_at IS NULL
  AND created_at >= sqlc.arg('from')
  AND created_at < sqlc.arg('to');

-- Série temporal do gráfico. generate_series preenche os períodos sem download
-- com zero: sem isso o gráfico une dois pontos distantes com uma reta e inventa
-- movimento que não existiu.
--
-- Os baldes são truncados NO FUSO DE SÃO PAULO, e não em UTC. É o mesmo corte
-- usado pelo limite diário; em UTC, o "dia" do gráfico começaria às 21h do dia
-- anterior e não bateria com o contador que o usuário vê.
-- name: AdminDownloadsTimeSeries :many
SELECT
  b.inicio::timestamptz AS bucket,
  COALESCE(COUNT(d.id), 0)::bigint AS total,
  COALESCE(COUNT(d.id) FILTER (WHERE d.status = 'COMPLETED'), 0)::bigint AS completed,
  COALESCE(COUNT(d.id) FILTER (WHERE d.status = 'FAILED'), 0)::bigint AS failed,
  COALESCE(SUM(d.file_size_bytes), 0)::bigint AS transferred_bytes
FROM (
  SELECT
    periodo AT TIME ZONE 'America/Sao_Paulo' AS inicio,
    (periodo + ('1 ' || sqlc.arg('granularity')::text)::interval)
      AT TIME ZONE 'America/Sao_Paulo' AS fim
  FROM generate_series(
    date_trunc(
      sqlc.arg('granularity')::text,
      sqlc.arg('from')::timestamptz AT TIME ZONE 'America/Sao_Paulo'
    ),
    -- O limite superior é o último balde que COMEÇA dentro do período. Sem o
    -- recuo, generate_series devolveria um balde extra começando exatamente no
    -- fim do intervalo, e o gráfico exibiria uma coluna de um dia que ainda não
    -- aconteceu.
    date_trunc(
      sqlc.arg('granularity')::text,
      (sqlc.arg('to')::timestamptz - INTERVAL '1 microsecond') AT TIME ZONE 'America/Sao_Paulo'
    ),
    ('1 ' || sqlc.arg('granularity')::text)::interval
  ) AS periodo
) b
LEFT JOIN downloads d
  ON d.deleted_at IS NULL
  AND d.created_at >= b.inicio
  AND d.created_at < b.fim
  -- Semana e mês truncam para trás, então o primeiro balde pode começar antes
  -- do período pedido. Sem estes dois limites ele somaria downloads anteriores
  -- ao intervalo, e a primeira barra apareceria inflada.
  AND d.created_at >= sqlc.arg('from')::timestamptz
  AND d.created_at < sqlc.arg('to')::timestamptz
GROUP BY b.inicio
ORDER BY b.inicio;

-- Armazenamento. "Atual" é o que ainda ocupa espaço no bucket: concluído, não
-- removido e dentro da validade. "Histórico" é tudo que já passou por lá.
-- name: AdminStorageSummary :one
SELECT
  COALESCE(SUM(file_size_bytes) FILTER (
    WHERE status = 'COMPLETED'
      AND deleted_at IS NULL
      AND (expires_at IS NULL OR expires_at > now())
  ), 0)::bigint AS stored_bytes,
  COUNT(*) FILTER (
    WHERE status = 'COMPLETED'
      AND deleted_at IS NULL
      AND (expires_at IS NULL OR expires_at > now())
  )::bigint AS stored_files,
  COALESCE(SUM(file_size_bytes), 0)::bigint AS lifetime_bytes,
  COALESCE(SUM(file_size_bytes) FILTER (
    WHERE status = 'EXPIRED' OR deleted_at IS NOT NULL
  ), 0)::bigint AS freed_bytes,
  COUNT(*) FILTER (WHERE status = 'EXPIRED' OR deleted_at IS NOT NULL)::bigint AS freed_files
FROM downloads;

-- name: AdminDownloadsByFormat :many
SELECT
  format,
  COUNT(*)::bigint AS total,
  COALESCE(SUM(file_size_bytes), 0)::bigint AS transferred_bytes
FROM downloads
WHERE deleted_at IS NULL
  AND created_at >= sqlc.arg('from')
  AND created_at < sqlc.arg('to')
GROUP BY format
ORDER BY total DESC;

-- name: AdminTopUsers :many
SELECT
  u.id,
  u.full_name,
  u.email,
  u.plan,
  COUNT(d.id)::bigint AS downloads_total,
  COALESCE(SUM(d.file_size_bytes), 0)::bigint AS transferred_bytes
FROM downloads d
JOIN users u ON u.id = d.user_id
WHERE d.deleted_at IS NULL
  AND u.deleted_at IS NULL
  AND d.created_at >= sqlc.arg('from')
  AND d.created_at < sqlc.arg('to')
GROUP BY u.id
ORDER BY downloads_total DESC
LIMIT $1;

-- Totais que não dependem do período escolhido.
-- name: AdminPlatformTotals :one
SELECT
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL)::bigint AS users_total,
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND active = true)::bigint AS users_active,
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND plan = 'premium')::bigint AS users_premium,
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= now() - INTERVAL '30 days')::bigint AS users_new_30d,
  (SELECT COUNT(*) FROM downloads WHERE deleted_at IS NULL)::bigint AS downloads_total;
