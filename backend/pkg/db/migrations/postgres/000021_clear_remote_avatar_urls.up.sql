UPDATE users
SET avatar_url = NULL,
    updated_at = NOW()
WHERE avatar_url ~* '^https?://';
