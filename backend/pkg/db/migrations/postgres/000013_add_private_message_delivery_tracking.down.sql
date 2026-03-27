DROP INDEX IF EXISTS idx_private_messages_recipient_delivery;

ALTER TABLE private_messages
    DROP COLUMN IF EXISTS delivered_at;
