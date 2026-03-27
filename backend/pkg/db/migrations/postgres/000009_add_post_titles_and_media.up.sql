ALTER TABLE posts
    ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS post_media (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    storage_path TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (post_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_post_media_post_id ON post_media(post_id, sort_order);
