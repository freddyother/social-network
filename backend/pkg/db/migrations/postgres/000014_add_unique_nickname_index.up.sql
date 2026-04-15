CREATE UNIQUE INDEX IF NOT EXISTS idx_users_nickname_unique
    ON users (LOWER(BTRIM(nickname)))
    WHERE nickname IS NOT NULL AND BTRIM(nickname) <> '';
