-- name: CreateYacht :one
INSERT INTO yachts (owner_id, name, registration_no, yacht_type) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetYacht :one
SELECT * FROM yachts WHERE id = $1 AND owner_id = $2 AND org_id IS NULL;

-- name: ListYachts :many
SELECT * FROM yachts WHERE owner_id = $1 AND org_id IS NULL ORDER BY name, id LIMIT $2 OFFSET $3;

-- name: CountYachts :one
SELECT COUNT(*)::BIGINT FROM yachts WHERE owner_id = $1 AND org_id IS NULL;

-- name: UpdateYacht :exec
UPDATE yachts SET name = $1, registration_no = $2, yacht_type = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4 AND owner_id = $5 AND org_id IS NULL;

-- name: DeleteYacht :exec
DELETE FROM yachts WHERE id = $1 AND owner_id = $2 AND org_id IS NULL;

-- name: GetYachtByName :one
SELECT * FROM yachts WHERE owner_id = $1 AND name = $2 AND org_id IS NULL;

-- name: CreateOrgYacht :one
INSERT INTO yachts (owner_id, org_id, name, registration_no, yacht_type) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetOrgYacht :one
SELECT * FROM yachts WHERE id = $1 AND org_id = $2;

-- name: ListOrgYachts :many
SELECT * FROM yachts WHERE org_id = $1 ORDER BY name, id LIMIT $2 OFFSET $3;

-- name: CountOrgYachts :one
SELECT COUNT(*)::BIGINT FROM yachts WHERE org_id = $1;

-- name: UpdateOrgYacht :exec
UPDATE yachts SET name = $1, registration_no = $2, yacht_type = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4 AND org_id = $5;

-- name: DeleteOrgYacht :exec
DELETE FROM yachts WHERE id = $1 AND org_id = $2;
