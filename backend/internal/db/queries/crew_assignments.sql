-- name: CreateCrewAssignment :one
INSERT INTO crew_assignments (cruise_id, crew_member_id, role, patent_number) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListCruiseCrewAssignments :many
SELECT ca.id, ca.cruise_id, ca.crew_member_id, ca.role, ca.patent_number, ca.created_at, cm.full_name, cm.email
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
JOIN cruises c ON c.id = ca.cruise_id
WHERE ca.cruise_id = $1
  AND c.owner_id = $2
ORDER BY cm.full_name;

-- name: DeleteCrewAssignment :exec
DELETE FROM crew_assignments
WHERE crew_assignments.id = $1
  AND crew_assignments.cruise_id IN (SELECT cruises.id FROM cruises WHERE cruises.owner_id = $2);

-- name: GetCrewMemberCruises :many
SELECT c.*, ca.role
FROM crew_assignments ca
JOIN cruises c ON c.id = ca.cruise_id
WHERE ca.crew_member_id = $1
ORDER BY c.year DESC, c.embark_date DESC;

-- name: GetCrewMemberStats :one
SELECT
    COUNT(*)::BIGINT AS cruise_count,
    COALESCE(SUM(c.hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(c.miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(c.days), 0)::BIGINT AS total_days
FROM crew_assignments ca
JOIN cruises c ON c.id = ca.cruise_id
WHERE ca.crew_member_id = $1;
