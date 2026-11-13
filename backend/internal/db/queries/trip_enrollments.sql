-- name: CreateTripEnrollment :one
-- status is 'pending', or 'waitlisted' when the leg's places are taken.
INSERT INTO trip_enrollments (trip_id, user_id, note, status)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListTripEnrollments :many
SELECT te.id, te.trip_id, te.user_id, te.note, te.status, te.created_at, te.updated_at,
       u.name AS user_name, u.email AS user_email
FROM trip_enrollments te
JOIN users u ON u.id = te.user_id
WHERE te.trip_id = $1
ORDER BY te.created_at;

-- name: UpdateTripEnrollmentStatus :exec
UPDATE trip_enrollments SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: LockTripEnrollmentOfTrip :one
-- Row-locks one enrollment of a trip for a status change. No row when the
-- enrollment does not exist or belongs to another trip.
SELECT * FROM trip_enrollments
WHERE id = $1 AND trip_id = $2
FOR UPDATE;

-- name: DeleteTripEnrollment :exec
DELETE FROM trip_enrollments WHERE id = $1;

-- name: CountTripEnrollments :one
SELECT
    COUNT(*) FILTER (WHERE status = 'accepted')::BIGINT AS accepted,
    COUNT(*)::BIGINT AS total
FROM trip_enrollments
WHERE trip_id = $1;

-- name: GetUserTripEnrollment :one
SELECT * FROM trip_enrollments
WHERE trip_id = $1 AND user_id = $2;

-- name: DeleteTripEnrollmentsForTrip :exec
DELETE FROM trip_enrollments WHERE trip_id = $1;

-- name: CountAcceptedByCruiseTrip :many
-- Accepted crew per trip of a cruise: distinct users with an accepted direct
-- trip enrollment or an accepted cruise enrollment assigned to the trip.
SELECT t.id AS trip_id,
    (SELECT COUNT(*) FROM (
        SELECT te.user_id FROM trip_enrollments te WHERE te.trip_id = t.id AND te.status = 'accepted'
        UNION
        SELECT ce.user_id FROM cruise_enrollments ce WHERE ce.trip_id = t.id AND ce.status = 'accepted'
    ) accepted_users)::BIGINT AS accepted
FROM trips t
WHERE t.cruise_id = $1
ORDER BY t.id;

-- name: CountAcceptedForTrip :one
-- Accepted crew of one trip, counted the same way as CountAcceptedByCruiseTrip.
SELECT COUNT(*)::BIGINT AS accepted FROM (
    SELECT te.user_id FROM trip_enrollments te WHERE te.trip_id = $1 AND te.status = 'accepted'
    UNION
    SELECT ce.user_id FROM cruise_enrollments ce WHERE ce.trip_id = $1 AND ce.status = 'accepted'
) accepted_users;

-- name: CountOtherAcceptedForTrip :one
-- Accepted crew of one trip other than the given user, counted the same way
-- as CountAcceptedForTrip. Accepting that user's sign-up takes one more place.
SELECT COUNT(*)::BIGINT AS accepted FROM (
    SELECT te.user_id FROM trip_enrollments te
    WHERE te.trip_id = $1 AND te.status = 'accepted' AND te.user_id <> $2
    UNION
    SELECT ce.user_id FROM cruise_enrollments ce
    WHERE ce.trip_id = $1 AND ce.status = 'accepted' AND ce.user_id <> $2
) accepted_users;

-- name: CountTripPlaceUse :one
-- How one trip's places are used. taken counts distinct users with an
-- accepted or pending direct trip enrollment or cruise enrollment assigned
-- to the trip; waitlisted counts its waitlisted trip enrollments. A new
-- sign-up is waitlisted once taken reaches max_crew or anyone is waiting.
SELECT
    (SELECT COUNT(*) FROM (
        SELECT te.user_id FROM trip_enrollments te
        WHERE te.trip_id = sqlc.arg(trip_id)::BIGINT AND te.status IN ('accepted', 'pending')
        UNION
        SELECT ce.user_id FROM cruise_enrollments ce
        WHERE ce.trip_id = sqlc.arg(trip_id)::BIGINT AND ce.status IN ('accepted', 'pending')
    ) taken_users)::BIGINT AS taken,
    (SELECT COUNT(*) FROM trip_enrollments w
     WHERE w.trip_id = sqlc.arg(trip_id)::BIGINT AND w.status = 'waitlisted')::BIGINT AS waitlisted;

-- name: GetWaitlistPosition :one
-- 1-based position of a waitlisted trip enrollment among its trip's
-- waitlisted enrollments, ordered by sign-up time, then id.
SELECT COUNT(*)::BIGINT AS position
FROM trip_enrollments w
JOIN trip_enrollments me ON me.trip_id = w.trip_id
WHERE me.id = $1
  AND w.status = 'waitlisted'
  AND (w.created_at, w.id) <= (me.created_at, me.id);

-- name: CreateTripEnrollmentIfAbsent :one
-- Inserts a trip enrollment unless the user already has one for the trip;
-- returns no row (sql.ErrNoRows) when the enrollment already exists. status
-- is 'pending', or 'waitlisted' when the leg's places are taken.
INSERT INTO trip_enrollments (trip_id, user_id, note, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (trip_id, user_id) DO NOTHING
RETURNING *;

-- name: ListUserCruiseTripEnrollments :many
-- A user's trip (leg) enrollments across the trips of one cruise, in leg order.
SELECT te.*
FROM trip_enrollments te
JOIN trips t ON t.id = te.trip_id
WHERE t.cruise_id = $1 AND te.user_id = $2
ORDER BY t.embark_date ASC, t.id ASC;

-- name: LockCruiseTripStatuses :many
-- Locks a cruise's trips for the rest of the transaction, in id order, so a
-- leg cannot be cancelled or removed between validation and insert, and
-- concurrent sign-ups and accepts of one leg take its places one at a time.
-- FOR NO KEY UPDATE still lets foreign-key checks (KEY SHARE) through.
SELECT id, status, max_crew FROM trips
WHERE cruise_id = $1
ORDER BY id
FOR NO KEY UPDATE;

-- name: CountTripLinkedEnrollments :one
SELECT (
    (SELECT COUNT(*) FROM cruise_enrollments ce WHERE ce.trip_id = sqlc.arg(trip_id)::BIGINT)
    + (SELECT COUNT(*) FROM trip_enrollments te WHERE te.trip_id = sqlc.arg(trip_id)::BIGINT)
)::BIGINT AS linked;

-- name: LockTripForSignUp :one
-- Locks one trip for the rest of a sign-up or accept transaction, so the leg
-- cannot be cancelled, closed, completed or deleted between the check and
-- the write, and concurrent sign-ups and accepts of the leg take its places
-- one at a time. FOR NO KEY UPDATE still lets foreign-key checks (KEY SHARE)
-- through. open is true while the leg's sign-ups are switched on (it has an
-- enroll token). No row when the trip does not exist.
SELECT id, status, (enroll_token IS NOT NULL)::BOOLEAN AS open, max_crew
FROM trips
WHERE id = $1
FOR NO KEY UPDATE;

-- name: UpdateUserTripEnrollmentNote :one
-- Sets the note on the caller's own sign-up for a trip. No row when the
-- user has no sign-up for it.
UPDATE trip_enrollments SET note = $3, updated_at = CURRENT_TIMESTAMP
WHERE trip_id = $1 AND user_id = $2
RETURNING *;

-- name: WithdrawUserTripEnrollment :one
-- Withdraws the caller's own sign-up for a trip while it is pending or on
-- the waiting list. No row when the user has no such sign-up for it.
DELETE FROM trip_enrollments
WHERE trip_id = $1 AND user_id = $2 AND status IN ('pending', 'waitlisted')
RETURNING id;

-- name: ListCruiseTripEnrollments :many
-- Every trip (leg) enrollment of a cruise with the enrolled user, grouped by
-- leg in leg order, then by sign-up time.
SELECT te.id, te.trip_id, te.user_id, te.note, te.status, te.created_at, te.updated_at,
       u.name AS user_name, u.email AS user_email
FROM trip_enrollments te
JOIN trips t ON t.id = te.trip_id
JOIN users u ON u.id = te.user_id
WHERE t.cruise_id = $1
ORDER BY t.embark_date ASC, t.id ASC, te.created_at ASC, te.id ASC;
