-- +goose Up

-- A player account. Identity only; game state lives elsewhere.
CREATE TABLE users (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE
);

-- One SSH key may map to several users, so a person can keep more than one
-- character on the same key. The pair is what must be unique, not the key.
CREATE TABLE user_keys (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    public_key  TEXT NOT NULL,
    UNIQUE (fingerprint, user_id)
);

CREATE INDEX idx_user_keys_fingerprint ON user_keys(fingerprint);

-- +goose Down
DROP INDEX idx_user_keys_fingerprint;
DROP TABLE user_keys;
DROP TABLE users;
