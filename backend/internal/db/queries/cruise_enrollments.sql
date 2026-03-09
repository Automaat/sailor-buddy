-- name: GetCruiseByEnrollToken :one
SELECT c.id, c.name, c.year, c.embark_date, c.disembark_date, c.countries,
       c.start_port, c.end_port, c.description, c.max_crew, c.captain_name,
       c.image_photo_url
FROM cruises c
WHERE c.enroll_token = $1;

-- name: CreateCruiseEnrollment :one
INSERT INTO cruise_enrollments (cruise_id, user_id, note)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListCruiseEnrollments :many
SELECT ce.id, ce.cruise_id, ce.user_id, ce.note, ce.status, ce.created_at, ce.updated_at,
       u.name AS user_name, u.email AS user_email
FROM cruise_enrollments ce
JOIN users u ON u.id = ce.user_id
JOIN cruises c ON c.id = ce.cruise_id
WHERE ce.cruise_id = $1 AND c.owner_id = $2
ORDER BY ce.created_at;

-- name: UpdateEnrollmentStatus :exec
UPDATE cruise_enrollments ce SET
    status = $1,
    updated_at = CURRENT_TIMESTAMP
FROM cruises c
WHERE ce.id = $2 AND ce.cruise_id = c.id AND c.owner_id = $3;

-- name: DeleteCruiseEnrollment :exec
DELETE FROM cruise_enrollments ce
USING cruises c
WHERE ce.id = $1 AND ce.cruise_id = c.id AND c.owner_id = $2;

-- name: CountCruiseEnrollments :one
SELECT
    COUNT(*) FILTER (WHERE status = 'accepted')::BIGINT AS accepted,
    COUNT(*)::BIGINT AS total
FROM cruise_enrollments
WHERE cruise_id = $1;

-- name: GetUserEnrollment :one
SELECT * FROM cruise_enrollments
WHERE cruise_id = $1 AND user_id = $2;
