-- name: ListCruiseEnrollments :many
SELECT ce.id, ce.cruise_id, ce.user_id, ce.trip_id, ce.note, ce.status, ce.created_at, ce.updated_at,
       u.name AS user_name, u.email AS user_email,
       u.phone AS user_phone, u.patent_type AS user_patent_type,
       u.patent_number AS user_patent_number,
       t.name AS trip_name
FROM cruise_enrollments ce
JOIN users u ON u.id = ce.user_id
LEFT JOIN trips t ON t.id = ce.trip_id
WHERE ce.cruise_id = $1
ORDER BY ce.created_at;

-- name: UpdateCruiseEnrollmentStatus :exec
UPDATE cruise_enrollments SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: AssignCruiseEnrollmentToTrip :exec
UPDATE cruise_enrollments ce SET
    trip_id = $1,
    updated_at = CURRENT_TIMESTAMP
WHERE ce.id = $2
  AND ($1::BIGINT IS NULL OR EXISTS (
      SELECT 1 FROM trips t WHERE t.id = $1 AND t.cruise_id = ce.cruise_id
  ));

-- name: DeleteCruiseEnrollment :exec
DELETE FROM cruise_enrollments WHERE id = $1;

-- name: CountCruiseEnrollments :one
SELECT
    COUNT(*) FILTER (WHERE status = 'accepted')::BIGINT AS accepted,
    COUNT(*)::BIGINT AS total
FROM cruise_enrollments
WHERE cruise_id = $1;

-- name: GetUserCruiseEnrollment :one
SELECT * FROM cruise_enrollments
WHERE cruise_id = $1 AND user_id = $2;

-- name: GetUserCruiseEnrollmentForTrip :one
-- A user's cruise-level enrollment assigned to a trip. Cruise enrollments
-- predate per-leg sign-up; one assigned to a leg counts as the user's
-- sign-up for that leg.
SELECT * FROM cruise_enrollments
WHERE trip_id = $1 AND user_id = $2
ORDER BY id
LIMIT 1;
