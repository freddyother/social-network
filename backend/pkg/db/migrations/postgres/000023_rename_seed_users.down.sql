WITH seed_users AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        (ARRAY[
            'Ada', 'Grace', 'Linus', 'Margaret', 'Alan',
            'Katherine', 'Dennis', 'Barbara', 'Tim', 'Radia',
            'Ken', 'Frances', 'Donald', 'Evelyn', 'Guido',
            'Anita', 'James', 'Mary', 'Brendan', 'Sophie'
        ])[n] AS first_name,
        (ARRAY[
            'Rivera', 'Chen', 'Morgan', 'Patel', 'Santos',
            'Okafor', 'Nguyen', 'Kowalski', 'Garcia', 'Smith',
            'Ivanov', 'Dubois', 'Brown', 'Wilson', 'Rossi',
            'Hernandez', 'Taylor', 'Anderson', 'Murphy', 'Martin'
        ])[n] AS last_name
    FROM generate_series(1, 20) AS series(n)
)
UPDATE users AS u
SET
    email = 'demo.user' || seed_users.nn || '@example.test',
    first_name = seed_users.first_name,
    last_name = seed_users.last_name,
    nickname = 'demo' || seed_users.nn,
    about_me = 'Demo profile for timeline, groups, comments, and media testing.',
    updated_at = NOW()
FROM seed_users
WHERE u.id = 'demo-user-' || seed_users.nn;

WITH seed_posts AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn
    FROM generate_series(1, 20) AS series(n)
)
UPDATE posts AS p
SET
    title = 'Demo day ' || seed_posts.nn,
    body = 'A seeded demo post with an image, written so the feed has realistic content to browse.',
    updated_at = NOW()
FROM seed_posts
WHERE p.id = 'demo-post-' || seed_posts.nn;

UPDATE comments
SET
    body = 'Nice demo post. Leaving a seeded comment so this account has comment activity.',
    updated_at = NOW()
WHERE id LIKE 'demo-comment-%';

UPDATE group_posts
SET
    body = 'Seeded group update with images for the demo workspace.',
    updated_at = NOW()
WHERE id LIKE 'demo-group-post-%';
