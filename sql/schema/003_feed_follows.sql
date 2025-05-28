-- +goose Up
CREATE TABLE feed_follows (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  user_id UUID NOT NULL,
  feed_id UUID NOT NULL,
  CONSTRAINT feed_follows_user_id_fkey FOREIGN KEY (user_id)
    REFERENCES USERS(id)
    ON DELETE CASCADE,
  CONSTRAINT feed_follows_feed_id_fkey FOREIGN KEY (feed_id)
    REFERENCES FEEDS(id)
    ON DELETE CASCADE,
  CONSTRAINT unique_feed_follow UNIQUE(user_id, feed_id)
);

-- +goose Down
DROP TABLE feed_follows;