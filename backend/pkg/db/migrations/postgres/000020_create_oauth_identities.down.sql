DROP INDEX IF EXISTS idx_oauth_identities_email;
DROP INDEX IF EXISTS idx_oauth_identities_user_id;
DROP TABLE IF EXISTS oauth_identities;

UPDATE users
SET date_of_birth = DATE '1970-01-01'
WHERE date_of_birth IS NULL;

ALTER TABLE users
    ALTER COLUMN date_of_birth SET NOT NULL;
