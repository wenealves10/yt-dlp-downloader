-- name: CountDownloadsToday :one
SELECT COUNT(*) FROM downloads
WHERE user_id = $1
  AND created_at::date = CURRENT_DATE;


