WITH requested_users AS (
    SELECT *
    FROM (VALUES
        ('fred'::TEXT),
        ('gus'::TEXT),
        ('lorem'::TEXT),
        ('win'::TEXT)
    ) AS requested(handle)
),
target_users AS (
    SELECT DISTINCT ON (requested_users.handle)
        requested_users.handle,
        u.id AS user_id
    FROM requested_users
    INNER JOIN users u ON (
        LOWER(BTRIM(COALESCE(u.nickname, ''))) = requested_users.handle
        OR LOWER(BTRIM(u.email)) = requested_users.handle
        OR (
            requested_users.handle = 'fred'
            AND LOWER(BTRIM(COALESCE(u.nickname, ''))) = 'fre'
        )
    )
    ORDER BY
        requested_users.handle,
        CASE
            WHEN LOWER(BTRIM(COALESCE(u.nickname, ''))) = requested_users.handle THEN 0
            WHEN LOWER(BTRIM(u.email)) = requested_users.handle THEN 1
            ELSE 2
        END,
        u.created_at ASC,
        u.id ASC
),
users_needing_seed_group AS (
    SELECT target_users.handle, target_users.user_id
    FROM target_users
    WHERE NOT EXISTS (
        SELECT 1
        FROM group_memberships gm
        INNER JOIN groups g ON g.id = gm.group_id
        WHERE
            gm.user_id = target_users.user_id
            AND g.id LIKE 'demo-group-%'
    )
),
candidate_groups AS (
    SELECT
        users_needing_seed_group.user_id,
        g.id AS group_id,
        ROW_NUMBER() OVER (
            PARTITION BY users_needing_seed_group.user_id
            ORDER BY md5(users_needing_seed_group.user_id || ':' || g.id)
        ) AS rank
    FROM users_needing_seed_group
    CROSS JOIN groups g
    WHERE
        g.id LIKE 'demo-group-%'
        AND NOT EXISTS (
            SELECT 1
            FROM group_memberships gm
            WHERE
                gm.group_id = g.id
                AND gm.user_id = users_needing_seed_group.user_id
        )
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
    'member',
    NOW()
FROM candidate_groups
WHERE rank = 1
ON CONFLICT (group_id, user_id) DO NOTHING;
