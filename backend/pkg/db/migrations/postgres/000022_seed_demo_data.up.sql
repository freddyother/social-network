-- Demo seed data. All seeded users can sign in with password: 12345678
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
INSERT INTO users (
    id,
    email,
    password_hash,
    first_name,
    last_name,
    date_of_birth,
    avatar_url,
    nickname,
    about_me,
    profile_visibility,
    theme_preference,
    created_at,
    updated_at
)
SELECT
    'demo-user-' || nn,
    'demo.user' || nn || '@example.test',
    '$2b$10$cdcV7Kq6W1LHEC4UBd7g6eHyb.7X3ZgDHVgjFKbITh0ay6nzesaiO',
    first_name,
    last_name,
    (DATE '1982-01-01' + (n * INTERVAL '173 days'))::DATE,
    NULL,
    'demo' || nn,
    'Demo profile for timeline, groups, comments, and media testing.',
    CASE WHEN n IN (5, 12, 18) THEN 'private' ELSE 'public' END,
    (ARRAY['nexo-blue', 'nexo-ice', 'graphite-gold', 'nexo-cloud', 'nexo-harbor'])[((n - 1) % 5) + 1],
    NOW() - ((25 - n) * INTERVAL '1 day'),
    NOW() - ((25 - n) * INTERVAL '1 day')
FROM seed_users
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    date_of_birth = EXCLUDED.date_of_birth,
    nickname = EXCLUDED.nickname,
    about_me = EXCLUDED.about_me,
    profile_visibility = EXCLUDED.profile_visibility,
    theme_preference = EXCLUDED.theme_preference,
    updated_at = EXCLUDED.updated_at;

WITH seed_posts AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        'https://picsum.photos/seed/nexo-personal-' || LPAD(n::TEXT, 2, '0') || '/1200/800' AS image_url
    FROM generate_series(1, 20) AS series(n)
)
INSERT INTO posts (
    id,
    author_id,
    title,
    body,
    image_url,
    privacy,
    created_at,
    updated_at
)
SELECT
    'demo-post-' || nn,
    'demo-user-' || nn,
    'Demo day ' || nn,
    'A seeded demo post with an image, written so the feed has realistic content to browse.',
    image_url,
    'public',
    NOW() - ((20 - n) * INTERVAL '6 hours'),
    NOW() - ((20 - n) * INTERVAL '6 hours')
FROM seed_posts
ON CONFLICT (id) DO UPDATE SET
    author_id = EXCLUDED.author_id,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    image_url = EXCLUDED.image_url,
    privacy = EXCLUDED.privacy,
    updated_at = EXCLUDED.updated_at;

WITH seed_media AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        'https://picsum.photos/seed/nexo-personal-' || LPAD(n::TEXT, 2, '0') || '/1200/800' AS image_url
    FROM generate_series(1, 20) AS series(n)
)
INSERT INTO post_media (
    id,
    post_id,
    storage_path,
    sort_order,
    created_at
)
SELECT
    'demo-post-media-' || nn || '-01',
    'demo-post-' || nn,
    image_url,
    1,
    NOW() - ((20 - n) * INTERVAL '6 hours')
FROM seed_media
ON CONFLICT (id) DO UPDATE SET
    post_id = EXCLUDED.post_id,
    storage_path = EXCLUDED.storage_path,
    sort_order = EXCLUDED.sort_order;

WITH seed_comments AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        LPAD(((n % 20) + 1)::TEXT, 2, '0') AS target_nn
    FROM generate_series(1, 20) AS series(n)
)
INSERT INTO comments (
    id,
    post_id,
    author_id,
    body,
    image_url,
    parent_comment_id,
    created_at,
    updated_at
)
SELECT
    'demo-comment-' || nn,
    'demo-post-' || target_nn,
    'demo-user-' || nn,
    'Nice demo post. Leaving a seeded comment so this account has comment activity.',
    NULL,
    NULL,
    NOW() - ((20 - n) * INTERVAL '5 hours'),
    NOW() - ((20 - n) * INTERVAL '5 hours')
FROM seed_comments
ON CONFLICT (id) DO UPDATE SET
    post_id = EXCLUDED.post_id,
    author_id = EXCLUDED.author_id,
    body = EXCLUDED.body,
    image_url = EXCLUDED.image_url,
    parent_comment_id = EXCLUDED.parent_comment_id,
    updated_at = EXCLUDED.updated_at;

WITH seed_groups AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        (((n - 1) * 2) % 20) + 1 AS creator_n,
        (ARRAY[
            'Urban Makers', 'Weekend Hikers', 'Book Circuit', 'Kitchen Lab', 'Indie Films',
            'Code Studio', 'Photo Walks', 'Fitness Crew', 'Language Cafe', 'Music Exchange'
        ])[n] AS title,
        (ARRAY[
            'A group for sharing small creative builds and project notes.',
            'Local walks, trail photos, and weekend route planning.',
            'Reading notes, recommendations, and friendly discussion threads.',
            'Recipes, meal prep experiments, and the occasional kitchen win.',
            'Film picks, watch parties, and short reviews.',
            'Small coding projects, debugging sessions, and learning logs.',
            'Casual photography walks and visual inspiration.',
            'Training plans, check-ins, and realistic fitness goals.',
            'Language practice, culture notes, and conversation prompts.',
            'Playlists, local shows, and music discovery.'
        ])[n] AS description
    FROM generate_series(1, 10) AS series(n)
)
INSERT INTO groups (
    id,
    creator_id,
    title,
    description,
    created_at,
    updated_at
)
SELECT
    'demo-group-' || nn,
    'demo-user-' || LPAD(creator_n::TEXT, 2, '0'),
    title,
    description,
    NOW() - ((14 - n) * INTERVAL '1 day'),
    NOW() - ((14 - n) * INTERVAL '1 day')
FROM seed_groups
ON CONFLICT (id) DO UPDATE SET
    creator_id = EXCLUDED.creator_id,
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    updated_at = EXCLUDED.updated_at;

WITH seed_groups AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        (((n - 1) * 2) % 20) + 1 AS creator_n
    FROM generate_series(1, 10) AS series(n)
),
seed_members AS (
    SELECT
        'demo-group-' || nn AS group_id,
        'demo-user-' || LPAD((((creator_n - 1 + member_offset) % 20) + 1)::TEXT, 2, '0') AS user_id,
        CASE WHEN member_offset = 0 THEN 'creator' ELSE 'member' END AS role,
        n,
        member_offset
    FROM seed_groups
    CROSS JOIN generate_series(0, 3) AS offsets(member_offset)
)
INSERT INTO group_memberships (
    group_id,
    user_id,
    role,
    joined_at
)
SELECT
    group_id,
    user_id,
    role,
    NOW() - ((14 - n) * INTERVAL '1 day') + (member_offset * INTERVAL '1 hour')
FROM seed_members
ON CONFLICT (group_id, user_id) DO UPDATE SET
    role = EXCLUDED.role,
    joined_at = EXCLUDED.joined_at;

WITH seed_group_posts AS (
    SELECT
        n,
        LPAD(n::TEXT, 2, '0') AS nn,
        (((n - 1) * 2) % 20) + 1 AS creator_n,
        'https://picsum.photos/seed/nexo-group-' || LPAD(n::TEXT, 2, '0') || '-cover/1400/900' AS image_url
    FROM generate_series(1, 10) AS series(n)
)
INSERT INTO group_posts (
    id,
    group_id,
    author_id,
    body,
    image_url,
    created_at,
    updated_at
)
SELECT
    'demo-group-post-' || nn,
    'demo-group-' || nn,
    'demo-user-' || LPAD(creator_n::TEXT, 2, '0'),
    'Seeded group update with images for the demo workspace.',
    image_url,
    NOW() - ((10 - n) * INTERVAL '7 hours'),
    NOW() - ((10 - n) * INTERVAL '7 hours')
FROM seed_group_posts
ON CONFLICT (id) DO UPDATE SET
    group_id = EXCLUDED.group_id,
    author_id = EXCLUDED.author_id,
    body = EXCLUDED.body,
    image_url = EXCLUDED.image_url,
    updated_at = EXCLUDED.updated_at;

WITH seed_group_media AS (
    SELECT
        group_n,
        LPAD(group_n::TEXT, 2, '0') AS group_nn,
        media_n,
        'https://picsum.photos/seed/nexo-group-' || LPAD(group_n::TEXT, 2, '0') || '-' || media_n::TEXT || '/1400/900' AS image_url
    FROM generate_series(1, 10) AS groups(group_n)
    CROSS JOIN generate_series(1, 2) AS media(media_n)
)
INSERT INTO group_post_media (
    id,
    group_post_id,
    storage_path,
    sort_order,
    created_at
)
SELECT
    'demo-group-post-media-' || group_nn || '-' || LPAD(media_n::TEXT, 2, '0'),
    'demo-group-post-' || group_nn,
    image_url,
    media_n,
    NOW() - ((10 - group_n) * INTERVAL '7 hours')
FROM seed_group_media
ON CONFLICT (id) DO UPDATE SET
    group_post_id = EXCLUDED.group_post_id,
    storage_path = EXCLUDED.storage_path,
    sort_order = EXCLUDED.sort_order;
