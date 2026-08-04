-- +goose Up

-- Optional, and deliberately not unique: a person may hold several characters,
-- so their accounts share one address the same way they share one SSH key.
-- Stored lowercased by the writer so lookups need no collation rules.
ALTER TABLE users ADD COLUMN email TEXT;

CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX idx_users_email;
ALTER TABLE users DROP COLUMN email;
