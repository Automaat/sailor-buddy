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

-- name: UpdateVoyageLog :execrows
-- Writes for an admin or the account linked to the voyage's captain person,
-- else updates no row. The check reads the row it updates: a captain change
-- committed before the statement refuses it, and one still open makes it
-- wait for that change and recheck the new captain.
UPDATE voyages lv SET
    hours_total = sqlc.arg(hours_total), hours_sail = sqlc.arg(hours_sail),
    hours_engine = sqlc.arg(hours_engine), hours_over_6bf = sqlc.arg(hours_over_6bf),
    miles = sqlc.arg(miles), days = sqlc.arg(days), tidal_waters = sqlc.arg(tidal_waters),
    captain_notes = sqlc.narg(captain_notes), yacht_id = sqlc.narg(yacht_id),
    track_url = sqlc.narg(track_url), voyage_card = sqlc.narg(voyage_card),
    updated_at = CURRENT_TIMESTAMP
WHERE lv.id = sqlc.arg(id)
  AND (sqlc.arg(caller_is_admin)::BOOLEAN OR lv.captain_person_id IN (
      SELECT cm.id FROM crew_members cm WHERE cm.user_id = sqlc.arg(caller_id)::BIGINT));

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
-- wins on overlap. Without include_hidden only visible voyages count; both
-- publication times are returned so the client can mark hidden voyages.
SELECT
    v.id, v.name, v.year, v.embark_date, v.disembark_date,
    y.name AS yacht_name,
    COALESCE(cp.user_id = sqlc.arg(user_id)::BIGINT, FALSE)::BOOLEAN AS as_captain,
    v.hours_total, v.hours_sail, v.hours_engine, v.miles, v.days,
    v.published_at, c.published_at AS cruise_published_at
FROM voyages v
JOIN cruises c ON c.id = v.cruise_id
LEFT JOIN yachts y ON y.id = v.yacht_id
LEFT JOIN crew_members cp ON cp.id = v.captain_person_id
WHERE (cp.user_id = sqlc.arg(user_id)::BIGINT
   OR EXISTS (
       SELECT 1 FROM crew_assignments ca
       JOIN crew_members cm ON cm.id = ca.crew_member_id
       WHERE ca.voyage_id = v.id AND cm.user_id = sqlc.arg(user_id)::BIGINT
   ))
  AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND c.published_at IS NOT NULL))
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
