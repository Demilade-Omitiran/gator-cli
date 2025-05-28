-- name: GetUsers :many
SELECT * FROM users
ORDER BY created_at DESC;