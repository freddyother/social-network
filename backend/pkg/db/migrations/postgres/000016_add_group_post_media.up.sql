CREATE TABLE IF NOT EXISTS group_post_media (
    id TEXT PRIMARY KEY,
    group_post_id TEXT NOT NULL REFERENCES group_posts(id) ON DELETE CASCADE,
    storage_path TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (group_post_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_group_post_media_post_id ON group_post_media(group_post_id, sort_order);

INSERT INTO group_post_media (id, group_post_id, storage_path, sort_order)
SELECT
    gp.id || '-cover',
    gp.id,
    gp.image_url,
    1
FROM group_posts gp
WHERE
    gp.image_url IS NOT NULL
    AND NOT EXISTS (
        SELECT 1
        FROM group_post_media gpm
        WHERE gpm.group_post_id = gp.id
    );
