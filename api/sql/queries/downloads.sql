-- name: CreateDownload :one
INSERT INTO downloads (
  id, user_id, original_url, format, status, thumbnail_url, file_url, expires_at, duration_seconds, error_message
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
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

-- name: UpdateDownloadStatus :exec
UPDATE downloads
SET status = $2, error_message = $3
WHERE id = $1;
