-- name: CreateCruise :one
INSERT INTO cruises (
    created_by, name, countries, description,
    image_logo_url, image_photo_url, image_route_url, series_id, edition,
    included, not_included, travel_notes, min_age
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: GetCruise :one
-- include_hidden is true for callers who may read hidden drafts (admins and
-- the captain group); others get no row for an unpublished cruise.
SELECT * FROM cruises
WHERE id = sqlc.arg(id)
  AND (sqlc.arg(include_hidden)::BOOLEAN OR published_at IS NOT NULL);

-- name: ListCruises :many
-- Newest first; a set after_id starts the page after that cruise. Without
-- include_hidden only published cruises are listed.
SELECT * FROM cruises
WHERE (sqlc.arg(include_hidden)::BOOLEAN OR published_at IS NOT NULL)
  AND (sqlc.narg(after_id)::BIGINT IS NULL OR id < sqlc.narg(after_id)::BIGINT)
ORDER BY id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountCruises :one
SELECT COUNT(*)::BIGINT FROM cruises
WHERE sqlc.arg(include_hidden)::BOOLEAN OR published_at IS NOT NULL;

-- name: UpdateCruise :exec
UPDATE cruises SET
    name = $1, countries = $2, description = $3,
    image_logo_url = $4, image_photo_url = $5, image_route_url = $6,
    series_id = $7, edition = $8, included = $9, not_included = $10,
    travel_notes = $11, min_age = $12,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $13;

-- name: DeleteCruise :exec
DELETE FROM cruises WHERE id = $1;

-- name: SetCruiseEnrollToken :exec
UPDATE cruises SET enroll_token = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: ClearCruiseEnrollToken :exec
UPDATE cruises SET enroll_token = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: LockCruiseEnrollToken :one
-- Share-locks the cruise for the rest of a share-link sign-up while it still
-- has the given enroll token, so clearing or regenerating the link waits for
-- the sign-up to commit and a sign-up after that sees the change. Taken
-- before LockCruiseTripStatuses, in the same cruise-then-trips order as
-- DELETE FROM cruises and trip completion. No row when the cruise no longer
-- has that token or is hidden; holding the lock also keeps an unpublish
-- waiting until the sign-up commits.
SELECT id FROM cruises
WHERE id = $1 AND enroll_token = $2 AND published_at IS NOT NULL
FOR SHARE;

-- name: GetCruiseByEnrollToken :one
SELECT id, name, countries, description, image_photo_url,
       included, not_included, travel_notes, min_age
FROM cruises WHERE enroll_token = $1 AND published_at IS NOT NULL;

-- name: ListCruiseTrips :many
-- Without include_hidden a leg is listed only while it and its cruise are
-- published.
SELECT * FROM trips t
WHERE t.cruise_id = sqlc.arg(cruise_id)
  AND (sqlc.arg(include_hidden)::BOOLEAN OR (t.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = t.cruise_id AND c.published_at IS NOT NULL)))
ORDER BY t.embark_date ASC, t.id ASC;

-- name: ListCruiseVoyages :many
-- Without include_hidden a voyage is listed only while it and its cruise are
-- published.
SELECT * FROM voyages v
WHERE v.cruise_id = sqlc.arg(cruise_id)
  AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)))
ORDER BY v.year DESC, v.embark_date DESC, v.id DESC;

-- name: PublishCruise :one
-- Publishes the cruise, keeping an earlier published_at.
UPDATE cruises SET
    published_at = COALESCE(published_at, CURRENT_TIMESTAMP),
    updated_at = CASE WHEN published_at IS NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE id = $1
RETURNING *;

-- name: UnpublishCruise :one
UPDATE cruises SET
    published_at = NULL,
    updated_at = CASE WHEN published_at IS NOT NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE id = $1
RETURNING *;

-- name: PublishStandaloneCruise :exec
-- Publishes a hidden cruise that belongs to no series, for a leg being
-- published: a standalone leg's wrapper cruise follows its leg.
UPDATE cruises SET published_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND published_at IS NULL AND series_id IS NULL;

-- name: PublishCruiseTrips :exec
UPDATE trips SET published_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE cruise_id = $1 AND published_at IS NULL;

-- name: PublishCruiseVoyages :exec
UPDATE voyages SET published_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE cruise_id = $1 AND published_at IS NULL;

-- name: ListStandaloneLegs :many
-- The "Bez serii" group: the trips and voyages of cruises without a series,
-- one row each, plus one row for such a cruise that has neither, so none
-- becomes unreachable. Newest first by embark date; an undated row is still
-- being planned, so it sorts before every dated one. Without include_hidden
-- a row is listed only while it and its cruise are published.
SELECT s.kind, s.id, s.name, s.cancelled, s.embark_date, s.disembark_date,
       s.start_port, s.end_port,
       s.start_port_latitude, s.start_port_longitude, s.end_port_latitude, s.end_port_longitude,
       s.published_at,
       s.cruise_id, s.cruise_name, s.cruise_published_at
FROM (
    SELECT 'trip'::TEXT AS kind, t.id, t.name, (t.status = 'cancelled') AS cancelled,
           t.embark_date, t.disembark_date, t.start_port, t.end_port,
           t.start_port_latitude, t.start_port_longitude, t.end_port_latitude, t.end_port_longitude,
           t.published_at,
           c.id AS cruise_id, c.name AS cruise_name, c.published_at AS cruise_published_at
    FROM trips t JOIN cruises c ON c.id = t.cruise_id
    WHERE c.series_id IS NULL
    UNION ALL
    SELECT 'voyage', v.id, v.name, FALSE,
           v.embark_date, v.disembark_date, v.start_port, v.end_port,
           v.start_port_latitude, v.start_port_longitude, v.end_port_latitude, v.end_port_longitude,
           v.published_at,
           c.id, c.name, c.published_at
    FROM voyages v JOIN cruises c ON c.id = v.cruise_id
    WHERE c.series_id IS NULL
    UNION ALL
    SELECT 'cruise', c.id, c.name, FALSE,
           NULL, NULL, NULL, NULL,
           NULL, NULL, NULL, NULL, c.published_at,
           c.id, c.name, c.published_at
    FROM cruises c
    WHERE c.series_id IS NULL
      AND NOT EXISTS (SELECT 1 FROM trips t WHERE t.cruise_id = c.id)
      AND NOT EXISTS (SELECT 1 FROM voyages v WHERE v.cruise_id = c.id)
) s
WHERE sqlc.arg(include_hidden)::BOOLEAN
   OR (s.published_at IS NOT NULL AND s.cruise_published_at IS NOT NULL)
ORDER BY NULLIF(s.embark_date, '') DESC NULLS FIRST, s.cruise_id DESC, s.kind, s.id DESC;
