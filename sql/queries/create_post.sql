-- name: CreatePost :one
INSERT INTO POSTS(id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;