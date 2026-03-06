-- name: CreateCruise :one
INSERT INTO cruises (
    owner_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days,
    captain_name, yacht_id, tidal_waters, cost_total, cost_per_person,
    image_logo_url, image_photo_url, image_route_url, description
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23) RETURNING *;

-- name: GetCruise :one
SELECT * FROM cruises WHERE id = $1 AND owner_id = $2;

-- name: ListCruises :many
SELECT * FROM cruises WHERE owner_id = $1 ORDER BY year DESC, embark_date DESC;

-- name: UpdateCruise :exec
UPDATE cruises SET
    name = $1, year = $2, embark_date = $3, disembark_date = $4, countries = $5,
    start_port = $6, end_port = $7, hours_total = $8, hours_sail = $9, hours_engine = $10,
    hours_over_6bf = $11, miles = $12, days = $13, captain_name = $14, yacht_id = $15,
    tidal_waters = $16, cost_total = $17, cost_per_person = $18,
    image_logo_url = $19, image_photo_url = $20, image_route_url = $21, description = $22,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $23 AND owner_id = $24;

-- name: DeleteCruise :exec
DELETE FROM cruises WHERE id = $1 AND owner_id = $2;

-- name: GetDashboardStats :one
SELECT
    COUNT(*)::BIGINT AS cruise_count,
    COALESCE(SUM(hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(days), 0)::BIGINT AS total_days,
    COALESCE(SUM(hours_sail), 0)::DOUBLE PRECISION AS total_hours_sail,
    COALESCE(SUM(hours_engine), 0)::DOUBLE PRECISION AS total_hours_engine
FROM cruises WHERE owner_id = $1;

-- name: GetCruisesByYear :many
SELECT
    year,
    COUNT(*)::BIGINT AS cruise_count,
    COALESCE(SUM(hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(days), 0)::BIGINT AS total_days
FROM cruises WHERE owner_id = $1 GROUP BY year ORDER BY year;
