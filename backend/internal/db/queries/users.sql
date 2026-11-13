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
  name = CASE WHEN EXCLUDED.name = '' OR users.onboarded_at IS NOT NULL
    THEN users.name ELSE EXCLUDED.name END,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: LinkFirebaseUIDByEmail :one
UPDATE users SET
  firebase_uid = sqlc.arg(firebase_uid),
  name = CASE WHEN onboarded_at IS NOT NULL THEN name
    ELSE COALESCE(NULLIF(sqlc.arg(new_name)::TEXT, ''), name) END,
  updated_at = CURRENT_TIMESTAMP
WHERE email = sqlc.arg(email)
  AND (firebase_uid IS NULL OR firebase_uid = sqlc.arg(firebase_uid))
RETURNING *;

-- name: UpdateUser :exec
UPDATE users SET name = $1, email = $2, avatar_url = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4;

-- name: UpdateUserProfile :one
UPDATE users SET
  first_name = sqlc.arg(first_name),
  last_name = sqlc.arg(last_name),
  name = sqlc.arg(first_name)::TEXT || ' ' || sqlc.arg(last_name)::TEXT,
  phone = sqlc.narg(phone),
  patent_type = sqlc.narg(patent_type),
  patent_number = sqlc.narg(patent_number),
  onboarded_at = COALESCE(onboarded_at, CURRENT_TIMESTAMP),
  updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CountUsers :one
SELECT COUNT(*)::BIGINT FROM users;

-- name: CountAdmins :one
SELECT COUNT(*)::BIGINT FROM users WHERE role = 'admin';

-- name: ListUsers :many
SELECT * FROM users ORDER BY role, name, id;

-- name: UpdateUserRole :exec
UPDATE users SET role = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: UpdateUserCaptain :exec
UPDATE users SET is_captain = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: UpdateUserMembership :exec
UPDATE users SET membership = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;
