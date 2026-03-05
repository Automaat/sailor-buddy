-- name: CreateCruise :one
INSERT INTO cruises (
    owner_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days,
    captain_name, yacht_id, tidal_waters, cost_total, cost_per_person,
    image_logo_url, image_photo_url, image_route_url, description
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetCruise :one
SELECT * FROM cruises WHERE id = ? AND owner_id = ?;

-- name: ListCruises :many
SELECT * FROM cruises WHERE owner_id = ? ORDER BY year DESC, embark_date DESC;

-- name: UpdateCruise :exec
UPDATE cruises SET
    name = ?, year = ?, embark_date = ?, disembark_date = ?, countries = ?,
    start_port = ?, end_port = ?, hours_total = ?, hours_sail = ?, hours_engine = ?,
    hours_over_6bf = ?, miles = ?, days = ?, captain_name = ?, yacht_id = ?,
    tidal_waters = ?, cost_total = ?, cost_per_person = ?,
    image_logo_url = ?, image_photo_url = ?, image_route_url = ?, description = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND owner_id = ?;

-- name: DeleteCruise :exec
DELETE FROM cruises WHERE id = ? AND owner_id = ?;

-- name: GetDashboardStats :one
SELECT
    COUNT(*) AS cruise_count,
    COALESCE(SUM(hours_total), 0) AS total_hours,
    COALESCE(SUM(miles), 0) AS total_miles,
    COALESCE(SUM(days), 0) AS total_days,
    COALESCE(SUM(hours_sail), 0) AS total_hours_sail,
    COALESCE(SUM(hours_engine), 0) AS total_hours_engine
FROM cruises WHERE owner_id = ?;

-- name: GetCruisesByYear :many
SELECT
    year,
    COUNT(*) AS cruise_count,
    COALESCE(SUM(hours_total), 0) AS total_hours,
    COALESCE(SUM(miles), 0) AS total_miles,
    COALESCE(SUM(days), 0) AS total_days
FROM cruises WHERE owner_id = ? GROUP BY year ORDER BY year;
