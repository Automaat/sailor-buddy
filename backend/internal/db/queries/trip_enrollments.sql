-- name: CreateTripEnrollment :one
INSERT INTO trip_enrollments (trip_id, user_id, note)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListTripEnrollments :many
SELECT te.id, te.trip_id, te.user_id, te.note, te.status, te.created_at, te.updated_at,
       u.name AS user_name, u.email AS user_email
FROM trip_enrollments te
JOIN users u ON u.id = te.user_id
WHERE te.trip_id = $1
ORDER BY te.created_at;

-- name: UpdateTripEnrollmentStatus :exec
UPDATE trip_enrollments SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

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
-- Accepted crew per trip of a cruise: direct trip enrollments plus cruise
-- enrollments assigned to the trip.
SELECT t.id AS trip_id,
    ((SELECT COUNT(*) FROM trip_enrollments te WHERE te.trip_id = t.id AND te.status = 'accepted')
   + (SELECT COUNT(*) FROM cruise_enrollments ce WHERE ce.trip_id = t.id AND ce.status = 'accepted'))::BIGINT AS accepted
FROM trips t
WHERE t.cruise_id = $1
ORDER BY t.id;

-- name: CountAcceptedForTrip :one
-- Accepted crew of one trip, counted the same way as CountAcceptedByCruiseTrip.
SELECT ((SELECT COUNT(*) FROM trip_enrollments te WHERE te.trip_id = $1 AND te.status = 'accepted')
      + (SELECT COUNT(*) FROM cruise_enrollments ce WHERE ce.trip_id = $1 AND ce.status = 'accepted'))::BIGINT AS accepted;
