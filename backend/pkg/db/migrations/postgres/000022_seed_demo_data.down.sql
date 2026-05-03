DELETE FROM group_post_media
WHERE id LIKE 'demo-group-post-media-%';

DELETE FROM group_posts
WHERE id LIKE 'demo-group-post-%';

DELETE FROM group_memberships
WHERE group_id LIKE 'demo-group-%';

DELETE FROM groups
WHERE id LIKE 'demo-group-%';

DELETE FROM comments
WHERE id LIKE 'demo-comment-%';

DELETE FROM post_media
WHERE id LIKE 'demo-post-media-%';

DELETE FROM posts
WHERE id LIKE 'demo-post-%';

DELETE FROM users
WHERE id LIKE 'demo-user-%';
