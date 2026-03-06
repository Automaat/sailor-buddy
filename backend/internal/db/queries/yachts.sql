-- name: CreateYacht :one
INSERT INTO yachts (owner_id, name, registration_no, yacht_type) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetYacht :one
SELECT * FROM yachts WHERE id = $1 AND owner_id = $2;

-- name: ListYachts :many
SELECT * FROM yachts WHERE owner_id = $1 ORDER BY name;

-- name: UpdateYacht :exec
UPDATE yachts SET name = $1, registration_no = $2, yacht_type = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4 AND owner_id = $5;

-- name: DeleteYacht :exec
DELETE FROM yachts WHERE id = $1 AND owner_id = $2;

-- name: GetYachtByName :one
SELECT * FROM yachts WHERE owner_id = $1 AND name = $2;
