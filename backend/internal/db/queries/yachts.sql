-- name: CreateYacht :one
INSERT INTO yachts (created_by, name, registration_no, yacht_type)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetYacht :one
SELECT * FROM yachts WHERE id = $1;

-- name: ListYachts :many
SELECT * FROM yachts ORDER BY name, id LIMIT $1 OFFSET $2;

-- name: CountYachts :one
SELECT COUNT(*)::BIGINT FROM yachts;

-- name: UpdateYacht :exec
UPDATE yachts SET name = $1, registration_no = $2, yacht_type = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $4;

-- name: DeleteYacht :exec
DELETE FROM yachts WHERE id = $1;

-- name: GetYachtByName :one
SELECT * FROM yachts WHERE name = $1;
