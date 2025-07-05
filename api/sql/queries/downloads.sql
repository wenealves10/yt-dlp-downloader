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
WHERE id = $1;

-- name: GetDownloadsByUser :many
SELECT * FROM downloads
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDownloadsByUser :one
SELECT COUNT(*) FROM downloads
WHERE user_id = $1;

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
DELETE FROM downloads
WHERE id = $1;

-- name: GetDownloadsExpired :many
SELECT *
FROM downloads
WHERE status = 'COMPLETED'
  AND expires_at IS NOT NULL
  AND expires_at <= NOW();
