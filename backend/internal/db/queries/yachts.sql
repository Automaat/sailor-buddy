-- name: CreateYacht :one
INSERT INTO yachts (owner_id, name, registration_no, yacht_type) VALUES (?, ?, ?, ?) RETURNING *;

-- name: GetYacht :one
SELECT * FROM yachts WHERE id = ? AND owner_id = ?;

-- name: ListYachts :many
SELECT * FROM yachts WHERE owner_id = ? ORDER BY name;

-- name: UpdateYacht :exec
UPDATE yachts SET name = ?, registration_no = ?, yacht_type = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND owner_id = ?;

-- name: DeleteYacht :exec
DELETE FROM yachts WHERE id = ? AND owner_id = ?;

-- name: GetYachtByName :one
SELECT * FROM yachts WHERE owner_id = ? AND name = ?;
