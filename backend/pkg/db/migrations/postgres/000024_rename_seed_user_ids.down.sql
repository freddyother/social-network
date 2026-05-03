DROP TABLE IF EXISTS seed_user_id_map;

CREATE TEMP TABLE seed_user_id_map (
    old_id TEXT PRIMARY KEY,
    new_id TEXT NOT NULL UNIQUE,
    nn TEXT NOT NULL,
    final_email TEXT NOT NULL,
    final_nickname TEXT NOT NULL
);

INSERT INTO seed_user_id_map (old_id, new_id, nn, final_email, final_nickname)
VALUES
    ('seed-user-01', 'demo-user-01', '01', 'luna.ortega@example.test', 'luna_ortega'),
    ('seed-user-02', 'demo-user-02', '02', 'mateo.silva@example.test', 'mateosilva'),
    ('seed-user-03', 'demo-user-03', '03', 'sofia.moreau@example.test', 'sofia_moreau'),
    ('seed-user-04', 'demo-user-04', '04', 'noah.bennett@example.test', 'noahbennett'),
    ('seed-user-05', 'demo-user-05', '05', 'isabel.costa@example.test', 'isabelcosta'),
    ('seed-user-06', 'demo-user-06', '06', 'elias.hart@example.test', 'eliashart'),
    ('seed-user-07', 'demo-user-07', '07', 'maya.singh@example.test', 'maya_singh'),
    ('seed-user-08', 'demo-user-08', '08', 'leo.navarro@example.test', 'leonavarro'),
    ('seed-user-09', 'demo-user-09', '09', 'amara.wilson@example.test', 'amara_wilson'),
    ('seed-user-10', 'demo-user-10', '10', 'theo.martin@example.test', 'theomartin'),
    ('seed-user-11', 'demo-user-11', '11', 'valeria.rossi@example.test', 'valeria_rossi'),
    ('seed-user-12', 'demo-user-12', '12', 'nico.alvarez@example.test', 'nicoalvarez'),
    ('seed-user-13', 'demo-user-13', '13', 'eva.kim@example.test', 'evakim'),
    ('seed-user-14', 'demo-user-14', '14', 'julian.price@example.test', 'julianprice'),
    ('seed-user-15', 'demo-user-15', '15', 'camila.lopez@example.test', 'camila_lopez'),
    ('seed-user-16', 'demo-user-16', '16', 'lucas.meyer@example.test', 'lucasmeyer'),
    ('seed-user-17', 'demo-user-17', '17', 'sara.dubois@example.test', 'saradubois'),
    ('seed-user-18', 'demo-user-18', '18', 'daniel.brooks@example.test', 'danielbrooks'),
    ('seed-user-19', 'demo-user-19', '19', 'nina.petrov@example.test', 'ninapetrov'),
    ('seed-user-20', 'demo-user-20', '20', 'oscar.reed@example.test', 'oscarreed');

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
    m.new_id,
    'seed.rollback.' || m.nn || '@example.test',
    u.password_hash,
    u.first_name,
    u.last_name,
    u.date_of_birth,
    u.avatar_url,
    'seed_rollback_' || m.nn,
    u.about_me,
    u.profile_visibility,
    u.theme_preference,
    u.created_at,
    NOW()
FROM users u
INNER JOIN seed_user_id_map m ON m.old_id = u.id
WHERE NOT EXISTS (
    SELECT 1 FROM users existing WHERE existing.id = m.new_id
);

UPDATE sessions s SET user_id = m.new_id FROM seed_user_id_map m WHERE s.user_id = m.old_id;
UPDATE follow_requests fr SET sender_id = m.new_id FROM seed_user_id_map m WHERE fr.sender_id = m.old_id;
UPDATE follow_requests fr SET recipient_id = m.new_id FROM seed_user_id_map m WHERE fr.recipient_id = m.old_id;
UPDATE followers f SET follower_id = m.new_id FROM seed_user_id_map m WHERE f.follower_id = m.old_id;
UPDATE followers f SET followee_id = m.new_id FROM seed_user_id_map m WHERE f.followee_id = m.old_id;
UPDATE posts p SET author_id = m.new_id FROM seed_user_id_map m WHERE p.author_id = m.old_id;
UPDATE post_audiences pa SET allowed_user_id = m.new_id FROM seed_user_id_map m WHERE pa.allowed_user_id = m.old_id;
UPDATE comments c SET author_id = m.new_id FROM seed_user_id_map m WHERE c.author_id = m.old_id;
UPDATE groups g SET creator_id = m.new_id FROM seed_user_id_map m WHERE g.creator_id = m.old_id;
UPDATE group_memberships gm SET user_id = m.new_id FROM seed_user_id_map m WHERE gm.user_id = m.old_id;
UPDATE group_invitations gi SET inviter_id = m.new_id FROM seed_user_id_map m WHERE gi.inviter_id = m.old_id;
UPDATE group_invitations gi SET invitee_id = m.new_id FROM seed_user_id_map m WHERE gi.invitee_id = m.old_id;
UPDATE group_join_requests gjr SET requester_id = m.new_id FROM seed_user_id_map m WHERE gjr.requester_id = m.old_id;
UPDATE notifications n SET user_id = m.new_id FROM seed_user_id_map m WHERE n.user_id = m.old_id;
UPDATE private_messages pm SET sender_id = m.new_id FROM seed_user_id_map m WHERE pm.sender_id = m.old_id;
UPDATE private_messages pm SET recipient_id = m.new_id FROM seed_user_id_map m WHERE pm.recipient_id = m.old_id;
UPDATE group_messages gm SET sender_id = m.new_id FROM seed_user_id_map m WHERE gm.sender_id = m.old_id;
UPDATE group_posts gp SET author_id = m.new_id FROM seed_user_id_map m WHERE gp.author_id = m.old_id;
UPDATE group_comments gc SET author_id = m.new_id FROM seed_user_id_map m WHERE gc.author_id = m.old_id;
UPDATE group_events ge SET creator_id = m.new_id FROM seed_user_id_map m WHERE ge.creator_id = m.old_id;
UPDATE event_responses er SET user_id = m.new_id FROM seed_user_id_map m WHERE er.user_id = m.old_id;
UPDATE password_reset_tokens prt SET user_id = m.new_id FROM seed_user_id_map m WHERE prt.user_id = m.old_id;
UPDATE post_reactions pr SET user_id = m.new_id FROM seed_user_id_map m WHERE pr.user_id = m.old_id;
UPDATE comment_reactions cr SET user_id = m.new_id FROM seed_user_id_map m WHERE cr.user_id = m.old_id;
UPDATE group_post_reactions gpr SET user_id = m.new_id FROM seed_user_id_map m WHERE gpr.user_id = m.old_id;
UPDATE group_comment_reactions gcr SET user_id = m.new_id FROM seed_user_id_map m WHERE gcr.user_id = m.old_id;
UPDATE oauth_identities oi SET user_id = m.new_id FROM seed_user_id_map m WHERE oi.user_id = m.old_id;

DELETE FROM users u
USING seed_user_id_map m
WHERE u.id = m.old_id;

UPDATE users u
SET
    email = m.final_email,
    nickname = m.final_nickname,
    updated_at = NOW()
FROM seed_user_id_map m
WHERE u.id = m.new_id;
