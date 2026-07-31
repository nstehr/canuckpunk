-- name: ListUsersByFingerprint :many
-- Every account reachable from one SSH key, oldest first so the menu order is
-- stable across logins.
SELECT u.*
FROM users u
JOIN user_keys k ON k.user_id = u.id
WHERE k.fingerprint = ?
ORDER BY u.id;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: CreateUser :one
INSERT INTO users (username) VALUES (?) RETURNING *;

-- name: LinkKeyToUser :exec
-- Re-linking an already known pair is a no-op, so a returning player can sign
-- in from the same key repeatedly.
INSERT INTO user_keys (user_id, fingerprint, public_key)
VALUES (?, ?, ?)
ON CONFLICT (fingerprint, user_id) DO NOTHING;
