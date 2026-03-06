-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, firebase_uid) VALUES (?, ?, '', ?) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = ?;

-- name: UpsertUserByFirebaseUID :one
INSERT INTO users (email, name, password_hash, firebase_uid)
VALUES (?, ?, '', ?)
ON CONFLICT(firebase_uid) DO UPDATE SET
  email = excluded.email,
  name = CASE WHEN excluded.name = '' THEN users.name ELSE excluded.name END,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpdateUser :exec
UPDATE users SET name = ?, email = ?, avatar_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;
