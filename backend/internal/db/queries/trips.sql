-- name: CreateTrip :one
INSERT INTO trips (
    created_by, cruise_id, name, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id, cost_total, cost_per_person, max_crew,
    image_logo_url, image_photo_url, image_route_url, description, status, captain_person_id,
    currency, cost_per_person_non_member, yacht_fund, deposit, sign_up_deadline
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
    $20, $21, $22, $23, $24
) RETURNING *;

-- name: GetTrip :one
-- include_hidden is true for callers who may read hidden drafts; others get
-- no row unless the trip and its cruise are both published.
SELECT * FROM trips t
WHERE t.id = sqlc.arg(id)
  AND (sqlc.arg(include_hidden)::BOOLEAN OR (t.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = t.cruise_id AND c.published_at IS NOT NULL)));

-- name: GetTripForUpdate :one
-- Row-locks the trip for the rest of the transaction. Trip completion takes
-- it first, so a concurrent leg enrollment waits on its trip lock and
-- then sees the trip gone instead of deadlocking with the deletes.
SELECT * FROM trips WHERE id = $1 FOR UPDATE;

-- name: LockTripCruise :one
-- Key-share-locks the cruise the trip belongs to. Trip completion takes it
-- before the trip lock, in the same order as DELETE FROM cruises (cruise,
-- then the trips foreign-key check), so the two cannot deadlock.
-- No row when the trip or its cruise is gone.
SELECT c.id FROM cruises c
WHERE c.id = (SELECT t.cruise_id FROM trips t WHERE t.id = $1)
FOR KEY SHARE OF c;

-- name: LockTripCaptain :one
-- Key-share-locks the trip's captain person. Trip completion takes it before
-- the trip lock, in the same order as deleting the person (the person, then
-- the trips ON DELETE SET NULL), so the two cannot deadlock.
-- No row when the trip has no captain person or either is gone.
SELECT cm.id FROM crew_members cm
WHERE cm.id = (SELECT t.captain_person_id FROM trips t WHERE t.id = $1)
FOR KEY SHARE OF cm;

-- name: LockTripYacht :one
-- Key-share-locks the trip's yacht. Trip completion takes it before the trip
-- lock, in the same order as deleting the yacht (the yacht, then the trips
-- foreign-key check), so the two cannot deadlock.
-- No row when the trip has no yacht or either is gone.
SELECT y.id FROM yachts y
WHERE y.id = (SELECT t.yacht_id FROM trips t WHERE t.id = $1)
FOR KEY SHARE OF y;

-- name: ListTrips :many
-- A set after_id starts the page after the cursor's sort key. The keyset
-- tuple sorts like the ORDER BY: a NULL embark_date last, then by id.
-- Without include_hidden a trip is listed only while it and its cruise are
-- published.
SELECT * FROM trips t
WHERE (sqlc.arg(include_hidden)::BOOLEAN OR (t.published_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM cruises c WHERE c.id = t.cruise_id AND c.published_at IS NOT NULL)))
  AND (sqlc.narg(after_id)::BIGINT IS NULL
   OR (t.embark_date IS NULL, COALESCE(t.embark_date, ''), t.id)
    > (sqlc.narg(after_embark_date)::TEXT IS NULL,
       COALESCE(sqlc.narg(after_embark_date)::TEXT, ''), sqlc.narg(after_id)::BIGINT))
ORDER BY t.embark_date ASC, t.id ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountTrips :one
SELECT COUNT(*)::BIGINT FROM trips t
WHERE sqlc.arg(include_hidden)::BOOLEAN OR (t.published_at IS NOT NULL AND EXISTS (
    SELECT 1 FROM cruises c WHERE c.id = t.cruise_id AND c.published_at IS NOT NULL));

-- name: UpdateTrip :exec
UPDATE trips SET
    name = $1, embark_date = $2, disembark_date = $3, countries = $4,
    start_port = $5, end_port = $6, captain_name = $7, yacht_id = $8,
    cost_total = $9, cost_per_person = $10, max_crew = $11,
    image_logo_url = $12, image_photo_url = $13, image_route_url = $14, description = $15,
    cruise_id = $16, captain_person_id = $17,
    currency = $18, cost_per_person_non_member = $19, yacht_fund = $20, deposit = $21,
    sign_up_deadline = $22,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $23;

-- name: DeleteTrip :exec
DELETE FROM trips WHERE id = $1;

-- name: CancelTrip :one
UPDATE trips SET status = 'cancelled', enroll_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'planned' RETURNING *;

-- name: SetTripEnrollToken :exec
UPDATE trips SET enroll_token = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: ClearTripEnrollToken :exec
UPDATE trips SET enroll_token = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: GetTripByEnrollToken :one
-- No row while the trip or its cruise is hidden.
SELECT t.id, t.name, t.embark_date, t.disembark_date, t.countries, t.start_port, t.end_port,
       t.description, t.max_crew, t.captain_name, t.image_photo_url,
       t.cost_per_person, t.currency, t.cost_per_person_non_member, t.yacht_fund, t.deposit,
       t.sign_up_deadline
FROM trips t
WHERE t.enroll_token = $1 AND t.published_at IS NOT NULL AND EXISTS (
    SELECT 1 FROM cruises c WHERE c.id = t.cruise_id AND c.published_at IS NOT NULL);

-- name: LockTripCruiseForPublish :one
-- Locks the cruise of the trip being published before the trip itself, the
-- order cruise publication with include_legs uses, so the two cannot
-- deadlock. No row when the trip or its cruise is gone.
SELECT c.id FROM cruises c
WHERE c.id = (SELECT t.cruise_id FROM trips t WHERE t.id = $1)
FOR NO KEY UPDATE OF c;

-- name: PublishTrip :one
-- Publishes the trip, keeping an earlier published_at.
UPDATE trips SET
    published_at = COALESCE(published_at, CURRENT_TIMESTAMP),
    updated_at = CASE WHEN published_at IS NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE id = $1
RETURNING *;

-- name: UnpublishTrip :one
UPDATE trips SET
    published_at = NULL,
    updated_at = CASE WHEN published_at IS NOT NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
WHERE id = $1
RETURNING *;
