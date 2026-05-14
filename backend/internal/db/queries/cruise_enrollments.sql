-- name: CreateCruiseEnrollment :one
INSERT INTO cruise_enrollments (cruise_id, user_id, note)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListCruiseEnrollments :many
SELECT ce.id, ce.cruise_id, ce.user_id, ce.trip_id, ce.note, ce.status, ce.created_at, ce.updated_at,
       u.name AS user_name, u.email AS user_email,
       t.name AS trip_name
FROM cruise_enrollments ce
JOIN users u ON u.id = ce.user_id
JOIN cruises c ON c.id = ce.cruise_id
LEFT JOIN trips t ON t.id = ce.trip_id
WHERE ce.cruise_id = $1 AND c.org_id = $2
ORDER BY ce.created_at;

-- name: UpdateCruiseEnrollmentStatus :exec
UPDATE cruise_enrollments ce SET
    status = $1,
    updated_at = CURRENT_TIMESTAMP
FROM cruises c
WHERE ce.id = $2 AND ce.cruise_id = c.id AND c.org_id = $3;

-- name: AssignCruiseEnrollmentToTrip :exec
UPDATE cruise_enrollments ce SET
    trip_id = $1,
    updated_at = CURRENT_TIMESTAMP
FROM cruises c
WHERE ce.id = $2 AND ce.cruise_id = c.id AND c.org_id = $3
  AND ($1::BIGINT IS NULL OR EXISTS (
      SELECT 1 FROM trips t WHERE t.id = $1 AND t.cruise_id = ce.cruise_id
  ));

-- name: DeleteCruiseEnrollment :exec
DELETE FROM cruise_enrollments ce
USING cruises c
WHERE ce.id = $1 AND ce.cruise_id = c.id AND c.org_id = $2;

-- name: CountCruiseEnrollments :one
SELECT
    COUNT(*) FILTER (WHERE status = 'accepted')::BIGINT AS accepted,
    COUNT(*)::BIGINT AS total
FROM cruise_enrollments
WHERE cruise_id = $1;

-- name: GetUserCruiseEnrollment :one
SELECT * FROM cruise_enrollments
WHERE cruise_id = $1 AND user_id = $2;
