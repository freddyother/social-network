ALTER TABLE users
    ADD COLUMN IF NOT EXISTS theme_preference TEXT NOT NULL DEFAULT 'nexo-blue'
    CHECK (theme_preference IN ('nexo-blue', 'nexo-ice', 'graphite-gold'));
