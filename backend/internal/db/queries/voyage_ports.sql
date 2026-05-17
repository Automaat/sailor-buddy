-- name: CreateVoyagePort :one
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListVoyagePorts :many
SELECT vp.* FROM voyage_ports vp
JOIN voyages v ON v.id = vp.voyage_id
WHERE vp.voyage_id = $1 AND v.owner_id = $2 AND v.org_id IS NULL
ORDER BY vp.position, vp.id;

-- name: DeleteVoyagePort :exec
DELETE FROM voyage_ports
WHERE voyage_ports.id = $1
  AND voyage_ports.voyage_id = $2
  AND voyage_ports.voyage_id IN (
      SELECT voyages.id FROM voyages WHERE voyages.owner_id = $3 AND voyages.org_id IS NULL
  );

-- name: ListOrgVoyagePorts :many
SELECT vp.* FROM voyage_ports vp
JOIN voyages v ON v.id = vp.voyage_id
WHERE vp.voyage_id = $1 AND v.org_id = $2
ORDER BY vp.position, vp.id;

-- name: DeleteOrgVoyagePort :exec
DELETE FROM voyage_ports
WHERE voyage_ports.id = $1
  AND voyage_ports.voyage_id = $2
  AND voyage_ports.voyage_id IN (
      SELECT voyages.id FROM voyages WHERE voyages.org_id = $3
  );
