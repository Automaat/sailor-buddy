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
-- Newest first; a set after_id starts the page after that cruise. The Rejsy
-- list orders cruises by their legs' dates itself. Without include_hidden
-- only published cruises are listed.
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

-- name: ListUserCruiseIDs :many
-- Cruises the user took part in: an accepted cruise or trip enrollment, a
-- leg captained by the account's person, or a crew-list entry linked to the
-- account, or one without an account that carries its email. Another
-- account's person never matches by email: it keeps the email that account
-- had when the person was created. Without include_hidden, a hidden cruise
-- and a cruise reached only through a hidden leg are left out.
WITH me AS (
    SELECT u.id AS me_id, u.email AS me_email FROM users u WHERE u.id = sqlc.arg(user_id)
),
my_crew AS (
    SELECT cm.id AS crew_id FROM crew_members cm, me
    WHERE cm.user_id = me.me_id
       OR (cm.user_id IS NULL AND cm.email IS NOT NULL AND lower(cm.email) = lower(me.me_email))
),
took_part AS (
    SELECT ce.cruise_id, TRUE AS leg_published FROM cruise_enrollments ce, me
    WHERE ce.user_id = me.me_id AND ce.status = 'accepted'
    UNION ALL
    SELECT t.cruise_id, t.published_at IS NOT NULL FROM trip_enrollments te JOIN trips t ON t.id = te.trip_id, me
    WHERE te.user_id = me.me_id AND te.status = 'accepted'
    UNION ALL
    SELECT t.cruise_id, t.published_at IS NOT NULL FROM trips t
    JOIN crew_members cm ON cm.id = t.captain_person_id, me
    WHERE cm.user_id = me.me_id
    UNION ALL
    SELECT v.cruise_id, v.published_at IS NOT NULL FROM voyages v
    JOIN crew_members cm ON cm.id = v.captain_person_id, me
    WHERE cm.user_id = me.me_id
    UNION ALL
    SELECT t.cruise_id, t.published_at IS NOT NULL FROM crew_assignments ca
    JOIN trips t ON t.id = ca.trip_id
    JOIN my_crew mc ON mc.crew_id = ca.crew_member_id
    UNION ALL
    SELECT v.cruise_id, v.published_at IS NOT NULL FROM crew_assignments ca
    JOIN voyages v ON v.id = ca.voyage_id
    JOIN my_crew mc ON mc.crew_id = ca.crew_member_id
)
SELECT DISTINCT tp.cruise_id FROM took_part tp
JOIN cruises c ON c.id = tp.cruise_id
WHERE sqlc.arg(include_hidden)::BOOLEAN OR (tp.leg_published AND c.published_at IS NOT NULL)
ORDER BY tp.cruise_id;
