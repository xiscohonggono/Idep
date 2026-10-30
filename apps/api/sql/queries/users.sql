-- name: GetUserByID :one
SELECT id, name, email, created_at
FROM users
WHERE id = $1 LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (id, name, email)
VALUES ($1, $2, $3)
RETURNING id, name, email, created_at;

