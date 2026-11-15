-- name: CreateVoyage :one
INSERT INTO voyages (
    created_by, cruise_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id,
    hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days, tidal_waters,
    cost_total, cost_per_person,
    image_logo_url, image_photo_url, image_route_url, description, captain_person_id,
    track_url, voyage_card, published_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28) RETURNING *;

-- name: GetVoyage :one
-- include_hidden is true for callers who may read hidden drafts; others get
-- no row unless the voyage and its cruise are both published.
SELECT * FROM voyages v
WHERE v.id = sqlc.arg(id)
  AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)));

-- name: LockVoyageForCaptain :one
-- Row-locks the voyage for a captain write, which rechecks the captain on
-- the locked row: a reassignment committed first is seen, a later one waits
-- for the write. FOR NO KEY UPDATE serializes captain writes on one voyage
-- and still lets foreign-key checks (KEY SHARE) of its ports through.
SELECT * FROM voyages WHERE id = $1 FOR NO KEY UPDATE;

-- name: ListVoyages :many
-- A set after_id starts the page after the cursor's sort key. The keyset
-- tuple sorts descending like the ORDER BY: a NULL year or embark_date first.
-- Without include_hidden a voyage is listed only while it and its cruise are
-- published.
SELECT * FROM voyages v
WHERE (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)))
  AND (sqlc.narg(after_id)::BIGINT IS NULL
   OR (v.year IS NULL, COALESCE(v.year, 0), v.embark_date IS NULL, COALESCE(v.embark_date, ''), v.id)
    < (sqlc.narg(after_year)::BIGINT IS NULL, COALESCE(sqlc.narg(after_year)::BIGINT, 0),
       sqlc.narg(after_embark_date)::TEXT IS NULL,
       COALESCE(sqlc.narg(after_embark_date)::TEXT, ''), sqlc.narg(after_id)::BIGINT))
ORDER BY v.year DESC, v.embark_date DESC, v.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountVoyages :one
SELECT COUNT(*)::BIGINT FROM voyages v
WHERE (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
    SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)));

-- name: UpdateVoyage :exec
UPDATE voyages SET
    name = $1, year = $2, embark_date = $3, disembark_date = $4, countries = $5,
    start_port = $6, end_port = $7, captain_name = $8, yacht_id = $9,
    hours_total = $10, hours_sail = $11, hours_engine = $12, hours_over_6bf = $13,
    miles = $14, days = $15, tidal_waters = $16,
    cost_total = $17, cost_per_person = $18,
    image_logo_url = $19, image_photo_url = $20, image_route_url = $21, description = $22,
    cruise_id = $23, captain_person_id = $24,
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
FROM voyages v
WHERE (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
    SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)));

-- name: GetVoyagesByYear :many
SELECT
    year,
    COUNT(*)::BIGINT AS voyage_count,
    COALESCE(SUM(hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(days), 0)::BIGINT AS total_days
FROM voyages v
WHERE (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
    SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)))
GROUP BY year ORDER BY year;

-- name: ListUserSeaTimeVoyages :many
-- Voyages the user sailed: as the captain through the account's person or
-- through a crew record linked to their account. One row per voyage; captain
-- wins on overlap. Without include_hidden only visible voyages count.
SELECT
    v.id, v.name, v.year, v.embark_date, v.disembark_date,
    y.name AS yacht_name,
    COALESCE(cp.user_id = sqlc.arg(user_id)::BIGINT, FALSE)::BOOLEAN AS as_captain,
    v.hours_total, v.hours_sail, v.hours_engine, v.miles, v.days
FROM voyages v
LEFT JOIN yachts y ON y.id = v.yacht_id
LEFT JOIN crew_members cp ON cp.id = v.captain_person_id
WHERE (cp.user_id = sqlc.arg(user_id)::BIGINT
   OR EXISTS (
       SELECT 1 FROM crew_assignments ca
       JOIN crew_members cm ON cm.id = ca.crew_member_id
       WHERE ca.voyage_id = v.id AND cm.user_id = sqlc.arg(user_id)::BIGINT
   ))
  AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)))
ORDER BY v.year DESC NULLS LAST, v.embark_date DESC NULLS LAST, v.id DESC;

-- name: LockVoyageCruiseForPublish :one
-- Locks the cruise of the voyage being published before the voyage itself,
-- the order cruise publication with include_legs uses, so the two cannot
-- deadlock. No row when the voyage or its cruise is gone.
SELECT c.id FROM cruises c
WHERE c.id = (SELECT v.cruise_id FROM voyages v WHERE v.id = $1)
FOR NO KEY UPDATE OF c;

-- name: PublishVoyage :one
-- Publishes the voyage, keeping an earlier published_at.
UPDATE voyages SET
    published_at = COALESCE(published_at, CURRENT_TIMESTAMP),
    updated_at = CASE WHEN published_at IS NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE id = $1
RETURNING *;

-- name: UnpublishVoyage :one
UPDATE voyages SET
    published_at = NULL,
    updated_at = CASE WHEN published_at IS NOT NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE id = $1
RETURNING *;
