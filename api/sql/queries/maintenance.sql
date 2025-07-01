-- name: CountDownloadsToday :one
SELECT COUNT(*) FROM downloads WHERE user_id = $1
  AND created_at >= TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE))
  AND created_at <  TIMEZONE('UTC', TIMEZONE('America/Sao_Paulo', CURRENT_DATE + INTERVAL '1 day'));



