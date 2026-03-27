ALTER TABLE private_messages
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_private_messages_recipient_delivery
    ON private_messages (recipient_id, delivered_at);
