-- name: CreateUser :one
INSERT INTO users (
    id,
    email,
    password_hash
) VALUES (
    sqlc.arg(id)::uuid,
    sqlc.arg(email),
    sqlc.arg(password_hash)
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE lower(email) = lower(sqlc.arg(email))
  AND deleted_at IS NULL
LIMIT 1;
