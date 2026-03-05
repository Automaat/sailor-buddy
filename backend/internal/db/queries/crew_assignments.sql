-- name: CreateCrewAssignment :one
INSERT INTO crew_assignments (cruise_id, crew_member_id, role, patent_number) VALUES (?, ?, ?, ?) RETURNING *;

-- name: ListCruiseCrewAssignments :many
SELECT ca.*, cm.full_name, cm.email
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
WHERE ca.cruise_id = ?
ORDER BY cm.full_name;

-- name: DeleteCrewAssignment :exec
DELETE FROM crew_assignments WHERE id = ?;

-- name: GetCrewMemberCruises :many
SELECT c.*, ca.role
FROM crew_assignments ca
JOIN cruises c ON c.id = ca.cruise_id
WHERE ca.crew_member_id = ?
ORDER BY c.year DESC, c.embark_date DESC;

-- name: GetCrewMemberStats :one
SELECT
    COUNT(*) AS cruise_count,
    COALESCE(SUM(c.hours_total), 0) AS total_hours,
    COALESCE(SUM(c.miles), 0) AS total_miles,
    COALESCE(SUM(c.days), 0) AS total_days
FROM crew_assignments ca
JOIN cruises c ON c.id = ca.cruise_id
WHERE ca.crew_member_id = ?;
