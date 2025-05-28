-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = NOW(), updated_at = $2
WHERE id = $1;