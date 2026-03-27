DROP INDEX IF EXISTS idx_comments_post_parent_created_at;
DROP INDEX IF EXISTS idx_comments_parent_comment_id;

ALTER TABLE comments
    DROP COLUMN IF EXISTS parent_comment_id,
    DROP COLUMN IF EXISTS updated_at;
