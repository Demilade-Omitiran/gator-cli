-- name: GetFeeds :many
SELECT F.id, F.name, F.url, U.name AS user_name, F.created_at, F.updated_at
FROM feeds F
LEFT JOIN users U ON F.user_id = U.id;