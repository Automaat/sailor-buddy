-- name: CreateVoyage :one
INSERT INTO voyages (
    created_by, cruise_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id,
    hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days, tidal_waters,
    cost_total, cost_per_person,
    image_logo_url, image_photo_url, image_route_url, description, captain_user_id,
    track_url, voyage_card
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27) RETURNING *;

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
    cruise_id = $23, captain_user_id = $24,
    track_url = $26, voyage_card = $27,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $25;

-- name: UpdateVoyageLog :exec
UPDATE voyages SET
    hours_total = $1, hours_sail = $2, hours_engine = $3, hours_over_6bf = $4,
    miles = $5, days = $6, tidal_waters = $7, captain_notes = $8,
    yacht_id = $10, track_url = $11, voyage_card = $12,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $9;

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

-- name: ListUserSeaTimeVoyages :many
-- Voyages the user sailed: as the assigned captain or through a crew record
-- linked to their account. One row per voyage; captain wins on overlap.
SELECT
    v.id, v.name, v.year, v.embark_date, v.disembark_date,
    y.name AS yacht_name,
    COALESCE(v.captain_user_id = sqlc.arg(user_id)::BIGINT, FALSE)::BOOLEAN AS as_captain,
    v.hours_total, v.hours_sail, v.hours_engine, v.miles, v.days
FROM voyages v
LEFT JOIN yachts y ON y.id = v.yacht_id
WHERE v.captain_user_id = sqlc.arg(user_id)::BIGINT
   OR EXISTS (
       SELECT 1 FROM crew_assignments ca
       JOIN crew_members cm ON cm.id = ca.crew_member_id
       WHERE ca.voyage_id = v.id AND cm.user_id = sqlc.arg(user_id)::BIGINT
   )
ORDER BY v.year DESC NULLS LAST, v.embark_date DESC NULLS LAST, v.id DESC;
