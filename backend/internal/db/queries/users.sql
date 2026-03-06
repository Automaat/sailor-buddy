-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, firebase_uid) VALUES ($1, $2, '', $3) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = $1;

-- name: UpsertUserByFirebaseUID :one
INSERT INTO users (email, name, password_hash, firebase_uid)
VALUES ($1, $2, '', $3)
ON CONFLICT(firebase_uid) DO UPDATE SET
  email = EXCLUDED.email,
  name = CASE WHEN EXCLUDED.name = '' THEN users.name ELSE EXCLUDED.name END,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: LinkFirebaseUIDByEmail :one
UPDATE users SET
  firebase_uid = sqlc.arg(firebase_uid),
  name = COALESCE(NULLIF(sqlc.arg(new_name)::TEXT, ''), name),
  updated_at = CURRENT_TIMESTAMP
WHERE email = sqlc.arg(email)
  AND (firebase_uid IS NULL OR firebase_uid = sqlc.arg(firebase_uid))
RETURNING *;

-- name: UpdateUser :exec
UPDATE users SET name = $1, email = $2, avatar_url = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4;
