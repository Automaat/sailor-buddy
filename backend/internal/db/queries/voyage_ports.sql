-- name: CreateVoyagePort :one
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListVoyagePorts :many
SELECT * FROM voyage_ports WHERE voyage_id = $1 ORDER BY position, id;

-- name: DeleteVoyagePort :exec
DELETE FROM voyage_ports WHERE id = $1 AND voyage_id = $2;

-- name: SetVoyagePortPosition :exec
UPDATE voyage_ports SET position = $3 WHERE id = $1 AND voyage_id = $2;
