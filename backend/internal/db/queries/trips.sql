-- name: CreateTrip :one
INSERT INTO trips (
    created_by, cruise_id, name, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id, cost_total, cost_per_person, max_crew,
    image_logo_url, image_photo_url, image_route_url, description, status, captain_user_id,
    currency, cost_per_person_non_member, yacht_fund, deposit
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
    $20, $21, $22, $23
) RETURNING *;

-- name: GetTrip :one
SELECT * FROM trips WHERE id = $1;

-- name: GetTripForUpdate :one
-- Row-locks the trip for the rest of the transaction. Trip completion takes
-- it first, so a concurrent leg enrollment waits on its FOR SHARE lock and
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

-- name: ListTrips :many
SELECT * FROM trips ORDER BY embark_date ASC, id ASC LIMIT $1 OFFSET $2;

-- name: CountTrips :one
SELECT COUNT(*)::BIGINT FROM trips;

-- name: UpdateTrip :exec
UPDATE trips SET
    name = $1, embark_date = $2, disembark_date = $3, countries = $4,
    start_port = $5, end_port = $6, captain_name = $7, yacht_id = $8,
    cost_total = $9, cost_per_person = $10, max_crew = $11,
    image_logo_url = $12, image_photo_url = $13, image_route_url = $14, description = $15,
    cruise_id = $16, captain_user_id = $17,
    currency = $18, cost_per_person_non_member = $19, yacht_fund = $20, deposit = $21,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $22;

-- name: DeleteTrip :exec
DELETE FROM trips WHERE id = $1;

-- name: CancelTrip :one
UPDATE trips SET status = 'cancelled', enroll_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'planned' RETURNING *;

-- name: GetTripStatus :one
SELECT status FROM trips WHERE id = $1;

-- name: SetTripEnrollToken :exec
UPDATE trips SET enroll_token = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: ClearTripEnrollToken :exec
UPDATE trips SET enroll_token = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: GetTripByEnrollToken :one
SELECT id, name, embark_date, disembark_date, countries, start_port, end_port,
       description, max_crew, captain_name, image_photo_url,
       cost_per_person, currency, cost_per_person_non_member, yacht_fund, deposit
FROM trips WHERE enroll_token = $1;
