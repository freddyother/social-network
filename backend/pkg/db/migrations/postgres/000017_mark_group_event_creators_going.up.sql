INSERT INTO event_responses (event_id, user_id, response)
SELECT ge.id, ge.creator_id, 'going'
FROM group_events ge
WHERE NOT EXISTS (
    SELECT 1
    FROM event_responses er
    WHERE er.event_id = ge.id AND er.user_id = ge.creator_id
);
