-- name: CreateUser :one
INSERT INTO users (
  id, full_name, email, hashed_password, plan, daily_limit, active, is_verified, password_changed_at
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: UpdateUserLoginInfo :exec
UPDATE users
SET last_login = now(), updated_at = now()
WHERE id = $1;

-- name: UpdateUser :exec
UPDATE users
SET
  full_name = COALESCE(sqlc.narg('full_name'),full_name),
  email = COALESCE(sqlc.narg('email'), email),
  photo_url = COALESCE(sqlc.narg('photo_url'), photo_url),
  plan = COALESCE(sqlc.narg('plan'), plan),
  daily_limit = COALESCE(sqlc.narg('daily_limit'), daily_limit),
  active = COALESCE(sqlc.narg('active'), active),
  is_verified = COALESCE(sqlc.narg('is_verified'), is_verified),
  hashed_password = COALESCE(sqlc.narg('hashed_password'), hashed_password),
  password_changed_at = COALESCE(sqlc.narg('password_changed_at'), password_changed_at),
  updated_at = now()
WHERE id = $1
RETURNING *;
