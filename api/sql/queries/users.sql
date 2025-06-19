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
