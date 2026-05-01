ALTER TABLE users
    ALTER COLUMN date_of_birth DROP NOT NULL;

CREATE TABLE IF NOT EXISTS oauth_identities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('google', 'apple')),
    provider_subject TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_subject)
);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_user_id
    ON oauth_identities (user_id);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_email
    ON oauth_identities (LOWER(BTRIM(email)));
