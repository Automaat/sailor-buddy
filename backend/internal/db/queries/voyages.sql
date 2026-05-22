-- name: CreateVoyage :one
INSERT INTO voyages (
    created_by, cruise_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id,
    hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days, tidal_waters,
    cost_total, cost_per_person,
    image_logo_url, image_photo_url, image_route_url, description
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24) RETURNING *;

-- name: GetVoyage :one
SELECT * FROM voyages WHERE id = $1;

-- name: ListVoyages :many
SELECT * FROM voyages ORDER BY year DESC, embark_date DESC, id DESC LIMIT $1 OFFSET $2;

-- name: CountVoyages :one
SELECT COUNT(*)::BIGINT FROM voyages;

-- name: UpdateVoyage :exec
UPDATE voyages SET
    name = $1, year = $2, embark_date = $3, disembark_date = $4, countries = $5,
    start_port = $6, end_port = $7, captain_name = $8, yacht_id = $9,
    hours_total = $10, hours_sail = $11, hours_engine = $12, hours_over_6bf = $13,
    miles = $14, days = $15, tidal_waters = $16,
    cost_total = $17, cost_per_person = $18,
    image_logo_url = $19, image_photo_url = $20, image_route_url = $21, description = $22,
    cruise_id = $23,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $24;

-- name: DeleteVoyage :exec
DELETE FROM voyages WHERE id = $1;

-- name: GetDashboardStats :one
SELECT
    COUNT(*)::BIGINT AS voyage_count,
    COALESCE(SUM(hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(days), 0)::BIGINT AS total_days,
    COALESCE(SUM(hours_sail), 0)::DOUBLE PRECISION AS total_hours_sail,
    COALESCE(SUM(hours_engine), 0)::DOUBLE PRECISION AS total_hours_engine
FROM voyages;

-- name: GetVoyagesByYear :many
SELECT
    year,
    COUNT(*)::BIGINT AS voyage_count,
    COALESCE(SUM(hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(days), 0)::BIGINT AS total_days
FROM voyages GROUP BY year ORDER BY year;
