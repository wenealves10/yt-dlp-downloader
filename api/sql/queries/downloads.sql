-- name: CreateDownload :one
INSERT INTO downloads (
  id, user_id, original_url, title, format, status, thumbnail_url, file_url, expires_at, duration_seconds, error_message
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetDownloadByID :one
SELECT * FROM downloads
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetDownloadsByUser :many
SELECT * FROM downloads
WHERE user_id = $1 
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDownloadsByUser :one
SELECT COUNT(*) FROM downloads
WHERE user_id = $1
  AND deleted_at IS NULL;

-- name: UpdateDownloadStatus :exec
UPDATE downloads
SET status = $2, error_message = $3
WHERE id = $1;


-- name: UpdateDownload :exec
UPDATE downloads
SET
  status = $2,
  file_url = $3,
  thumbnail_url = $4,
  expires_at = $5,
  error_message = $6
WHERE id = $1;

-- name: DeleteDownload :exec
UPDATE downloads
SET deleted_at = NOW()
WHERE id = $1;

-- name: GetDownloadsExpired :many
SELECT *
FROM downloads
WHERE status = 'COMPLETED'
  AND deleted_at IS NULL
  AND expires_at IS NOT NULL
  AND expires_at <= NOW();

-- Gravado pelo worker logo após o upload. É o que sustenta as métricas de
-- armazenamento do painel: sem isto, o total do storage fica zerado para sempre.
-- name: SetDownloadFileSize :exec
UPDATE downloads
SET file_size_bytes = $2
WHERE id = $1;

-- name: CreateMediaDownload :one
INSERT INTO downloads (
  id, user_id, original_url, title, format, status,
  thumbnail_url, duration_seconds, platform, provider,
  format_id, quality_label, uploader, total_bytes, format_height
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING *;

-- Gravado durante o download. Escreve pouco de propósito: o tempo real vai por
-- SSE, e isto é só o último estado conhecido para a tela reabrir no meio.
-- name: UpdateDownloadProgress :exec
UPDATE downloads
SET
  progress_percent = $2,
  downloaded_bytes = $3,
  total_bytes = GREATEST(total_bytes, $4),
  speed_bps = $5,
  eta_seconds = $6
WHERE id = $1;

-- name: MarkDownloadStarted :exec
UPDATE downloads
SET status = 'PROCESSING', started_at = now(), error_message = NULL
WHERE id = $1;

-- error_message é a mensagem PÚBLICA, lida pelo cliente. error_detail é o motivo
-- técnico e nunca sai para um usuário comum.
-- name: MarkDownloadFinished :exec
UPDATE downloads
SET status = $2,
    error_message = sqlc.narg('error_message'),
    error_detail = sqlc.narg('error_detail'),
    finished_at = now()
WHERE id = $1;

-- Cancelar é idempotente: o filtro de status está no SET, não no WHERE. Com ele
-- no WHERE, clicar em cancelar meio segundo depois de o download falhar sozinho
-- não casava linha nenhuma e a tela cuspia "não pode mais ser cancelado" — um
-- erro sobre uma corrida que o usuário não provocou nem pode evitar. Agora a
-- linha sempre volta: quem já terminou apenas mantém o status que tinha, e a
-- resposta diz qual é.
-- name: CancelDownload :one
UPDATE downloads
SET status = CASE
      WHEN status IN ('PENDING', 'PROCESSING', 'RETRYING')
        THEN 'CANCELED'::core.download_status
      ELSE status
    END,
    finished_at = CASE
      WHEN status IN ('PENDING', 'PROCESSING', 'RETRYING') THEN now()
      ELSE finished_at
    END
WHERE id = $1
  AND user_id = $2
  AND deleted_at IS NULL
RETURNING *;
