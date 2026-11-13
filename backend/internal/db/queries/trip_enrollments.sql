-- name: CreateTripEnrollment :one
INSERT INTO trip_enrollments (trip_id, user_id, note)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListTripEnrollments :many
SELECT te.id, te.trip_id, te.user_id, te.note, te.status, te.payment_status,
       te.created_at, te.updated_at,
       u.name AS user_name, u.email AS user_email
FROM trip_enrollments te
JOIN users u ON u.id = te.user_id
WHERE te.trip_id = $1
ORDER BY te.created_at;

-- name: UpdateTripEnrollmentStatus :exec
UPDATE trip_enrollments SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: UpdateTripEnrollmentPaymentStatus :one
-- Sets the manual payment status of one sign-up of a trip. The review
-- status is left unchanged. No row when the trip has no such enrollment.
UPDATE trip_enrollments SET payment_status = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2 AND trip_id = $3
RETURNING *;

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

-- name: CreateTripEnrollmentIfAbsent :one
-- Inserts a pending trip enrollment unless the user already has one for the
-- trip; returns no row (sql.ErrNoRows) when the enrollment already exists.
INSERT INTO trip_enrollments (trip_id, user_id, note)
VALUES ($1, $2, $3)
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
-- Share-locks a cruise's trips for the rest of the transaction, in id order,
-- so a leg cannot be cancelled or removed between validation and insert.
SELECT id, status FROM trips
WHERE cruise_id = $1
ORDER BY id
FOR SHARE;

-- name: CountTripLinkedEnrollments :one
SELECT (
    (SELECT COUNT(*) FROM cruise_enrollments ce WHERE ce.trip_id = sqlc.arg(trip_id)::BIGINT)
    + (SELECT COUNT(*) FROM trip_enrollments te WHERE te.trip_id = sqlc.arg(trip_id)::BIGINT)
)::BIGINT AS linked;

-- name: LockTripForSignUp :one
-- Share-locks one trip for the rest of a sign-up transaction, so the leg
-- cannot be cancelled, closed, completed or deleted between the open check
-- and the insert. open is true while the leg's sign-ups are switched on
-- (it has an enroll token). No row when the trip does not exist.
SELECT id, status, (enroll_token IS NOT NULL)::BOOLEAN AS open
FROM trips
WHERE id = $1
FOR SHARE;

-- name: UpdateUserTripEnrollmentNote :one
-- Sets the note on the caller's own sign-up for a trip. No row when the
-- user has no sign-up for it.
UPDATE trip_enrollments SET note = $3, updated_at = CURRENT_TIMESTAMP
WHERE trip_id = $1 AND user_id = $2
RETURNING *;

-- name: DeletePendingUserTripEnrollment :one
-- Withdraws the caller's own sign-up for a trip while it is still pending
-- and no payment is recorded on it. No row when the user has no such
-- sign-up for it.
DELETE FROM trip_enrollments
WHERE trip_id = $1 AND user_id = $2 AND status = 'pending' AND payment_status = 'unpaid'
RETURNING id;

-- name: ListCruiseTripEnrollments :many
-- Every trip (leg) enrollment of a cruise with the enrolled user, grouped by
-- leg in leg order, then by sign-up time.
SELECT te.id, te.trip_id, te.user_id, te.note, te.status, te.payment_status,
       te.created_at, te.updated_at,
       u.name AS user_name, u.email AS user_email
FROM trip_enrollments te
JOIN trips t ON t.id = te.trip_id
JOIN users u ON u.id = te.user_id
WHERE t.cruise_id = $1
ORDER BY t.embark_date ASC, t.id ASC, te.created_at ASC, te.id ASC;
