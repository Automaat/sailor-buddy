-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, firebase_uid) VALUES ($1, $2, '', $3) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = $1;

-- GetAuthUserByFirebaseUID is the auth middleware's read-only lookup. One
-- snapshot also reports whether the account has its person and whether any
-- admin exists, so a known, unchanged account needs no write.
-- name: GetAuthUserByFirebaseUID :one
SELECT sqlc.embed(users),
  EXISTS (SELECT 1 FROM crew_members cm WHERE cm.user_id = users.id)::BOOLEAN AS has_person,
  EXISTS (SELECT 1 FROM users admins WHERE admins.role = 'admin')::BOOLEAN AS admin_exists
FROM users
WHERE users.firebase_uid = $1;

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
-- The account's person mirrors its name, phone and patent number.
WITH person AS (
  UPDATE crew_members SET
    full_name = sqlc.arg(first_name)::TEXT || ' ' || sqlc.arg(last_name)::TEXT,
    phone = sqlc.narg(phone),
    patent_number = sqlc.narg(patent_number),
    updated_at = CURRENT_TIMESTAMP
  WHERE crew_members.user_id = sqlc.arg(id)
)
UPDATE users SET
  first_name = sqlc.arg(first_name),
  last_name = sqlc.arg(last_name),
  name = sqlc.arg(first_name)::TEXT || ' ' || sqlc.arg(last_name)::TEXT,
  phone = sqlc.narg(phone),
  patent_type = sqlc.narg(patent_type),
  patent_number = sqlc.narg(patent_number),
  onboarded_at = COALESCE(onboarded_at, CURRENT_TIMESTAMP),
  updated_at = CURRENT_TIMESTAMP
WHERE users.id = sqlc.arg(id)
RETURNING *;

-- name: CountUsers :one
SELECT COUNT(*)::BIGINT FROM users;

-- name: CountAdmins :one
SELECT COUNT(*)::BIGINT FROM users WHERE role = 'admin';

-- LockAdminRoles serializes role changes for the rest of the transaction.
-- Run it as its own statement, before LockAdmins, so the admin set is read
-- from a snapshot taken after the previous role change committed.
-- name: LockAdminRoles :exec
SELECT pg_advisory_xact_lock(hashtext('admin-roles'));

-- LockAdmins row-locks every admin, in id order. NO KEY UPDATE still
-- blocks other role writes but not foreign-key checks on admin rows (e.g.
-- decided_by), which would otherwise deadlock with a concurrent link.
-- name: LockAdmins :many
SELECT id FROM users WHERE role = 'admin' ORDER BY id FOR NO KEY UPDATE;

-- LockFirstAdmin serializes first-admin promotion for the rest of the
-- transaction. Run it as its own statement, before PromoteIfNoAdmin, so
-- the promotion reads a snapshot taken after the lock is held.
-- name: LockFirstAdmin :exec
SELECT pg_advisory_xact_lock(hashtext('first-admin'));

-- PromoteIfNoAdmin makes the user admin only while no other user is; it
-- reports 0 rows when another admin exists. Ignoring the user itself lets
-- a second concurrent first request of the promoted account report admin.
-- name: PromoteIfNoAdmin :execrows
UPDATE users SET role = 'admin', updated_at = CURRENT_TIMESTAMP
WHERE users.id = $1
  AND NOT EXISTS (
    SELECT 1 FROM users AS admins WHERE admins.role = 'admin' AND admins.id <> $1
  );

-- name: ListUsers :many
SELECT * FROM users ORDER BY role, name, id;

-- name: UpdateUserRole :exec
UPDATE users SET role = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: UpdateUserCaptain :exec
UPDATE users SET is_captain = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: UpdateUserMembership :exec
UPDATE users SET membership = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;
