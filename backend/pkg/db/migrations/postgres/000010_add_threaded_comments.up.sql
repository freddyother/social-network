ALTER TABLE comments
    ADD COLUMN IF NOT EXISTS parent_comment_id TEXT REFERENCES comments(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_comments_parent_comment_id
    ON comments(parent_comment_id, created_at);

CREATE INDEX IF NOT EXISTS idx_comments_post_parent_created_at
    ON comments(post_id, parent_comment_id, created_at);
