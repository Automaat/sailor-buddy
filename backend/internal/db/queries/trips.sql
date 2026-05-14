-- name: CreateTrip :one
INSERT INTO trips (
    owner_id, name, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id, cost_total, cost_per_person, max_crew,
    image_logo_url, image_photo_url, image_route_url, description, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17) RETURNING *;

-- name: GetTrip :one
SELECT * FROM trips WHERE id = $1 AND owner_id = $2 AND org_id IS NULL;

-- name: ListTrips :many
SELECT * FROM trips WHERE owner_id = $1 AND org_id IS NULL ORDER BY embark_date ASC;

-- name: UpdateTrip :exec
UPDATE trips SET
    name = $1, embark_date = $2, disembark_date = $3, countries = $4,
    start_port = $5, end_port = $6, captain_name = $7, yacht_id = $8,
    cost_total = $9, cost_per_person = $10, max_crew = $11,
    image_logo_url = $12, image_photo_url = $13, image_route_url = $14, description = $15,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $16 AND owner_id = $17 AND org_id IS NULL;

-- name: DeleteTrip :exec
DELETE FROM trips WHERE id = $1 AND owner_id = $2 AND org_id IS NULL;

-- name: CancelTrip :one
UPDATE trips SET status = 'cancelled', enroll_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND owner_id = $2 AND org_id IS NULL AND status = 'planned' RETURNING *;

-- name: GetTripStatus :one
SELECT status FROM trips WHERE id = $1;

-- name: SetTripEnrollToken :exec
UPDATE trips SET enroll_token = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2 AND owner_id = $3;

-- name: ClearTripEnrollToken :exec
UPDATE trips SET enroll_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND owner_id = $2;

-- name: GetTripByEnrollToken :one
SELECT id, name, embark_date, disembark_date, countries, start_port, end_port,
       description, max_crew, captain_name, image_photo_url
FROM trips WHERE enroll_token = $1;

-- name: CreateOrgTrip :one
INSERT INTO trips (
    owner_id, org_id, cruise_id, name, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id, cost_total, cost_per_person, max_crew,
    image_logo_url, image_photo_url, image_route_url, description, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19) RETURNING *;

-- name: GetOrgTrip :one
SELECT * FROM trips WHERE id = $1 AND org_id = $2;

-- name: ListOrgTrips :many
SELECT * FROM trips WHERE org_id = $1 ORDER BY embark_date ASC;

-- name: UpdateOrgTrip :exec
UPDATE trips SET
    name = $1, embark_date = $2, disembark_date = $3, countries = $4,
    start_port = $5, end_port = $6, captain_name = $7, yacht_id = $8,
    cost_total = $9, cost_per_person = $10, max_crew = $11,
    image_logo_url = $12, image_photo_url = $13, image_route_url = $14, description = $15,
    cruise_id = $16,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $17 AND org_id = $18;

-- name: DeleteOrgTrip :exec
DELETE FROM trips WHERE id = $1 AND org_id = $2;

-- name: CancelOrgTrip :one
UPDATE trips SET status = 'cancelled', enroll_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND org_id = $2 AND status = 'planned' RETURNING *;
