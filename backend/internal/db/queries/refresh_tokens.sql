-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?);

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE token_hash = ? AND revoked = 0 AND expires_at > CURRENT_TIMESTAMP;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ?;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < CURRENT_TIMESTAMP OR revoked = 1;
