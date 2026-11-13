-- name: CreateTripCrewAssignment :one
INSERT INTO crew_assignments (trip_id, crew_member_id, role, patent_number)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: CreateVoyageCrewAssignment :one
INSERT INTO crew_assignments (voyage_id, crew_member_id, role, patent_number)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListTripCrewAssignments :many
SELECT ca.id, ca.trip_id, ca.voyage_id, ca.crew_member_id, ca.role, ca.patent_number, ca.created_at,
       cm.full_name, cm.email
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
WHERE ca.trip_id = $1
ORDER BY cm.full_name;

-- name: ListVoyageCrewAssignments :many
SELECT ca.id, ca.trip_id, ca.voyage_id, ca.crew_member_id, ca.role, ca.patent_number, ca.created_at,
       cm.full_name, cm.email
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
WHERE ca.voyage_id = $1
ORDER BY cm.full_name;

-- name: DeleteTripCrewAssignment :exec
DELETE FROM crew_assignments WHERE id = $1 AND trip_id = $2;

-- name: DeleteVoyageCrewAssignment :exec
DELETE FROM crew_assignments WHERE id = $1 AND voyage_id = $2;

-- name: GetVoyageCrewAssignmentByMember :one
SELECT ca.*, cm.full_name, cm.patent_number AS member_patent
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
WHERE ca.voyage_id = $1 AND ca.crew_member_id = $2;

-- name: GetCrewMemberVoyages :many
SELECT v.*, ca.role
FROM crew_assignments ca
JOIN voyages v ON v.id = ca.voyage_id
WHERE ca.crew_member_id = $1
ORDER BY v.year DESC, v.embark_date DESC;

-- name: GetCrewMemberTrips :many
SELECT t.*, ca.role
FROM crew_assignments ca
JOIN trips t ON t.id = ca.trip_id
WHERE ca.crew_member_id = $1
ORDER BY t.embark_date ASC;

-- name: GetCrewMemberStats :one
SELECT
    COUNT(*)::BIGINT AS voyage_count,
    COALESCE(SUM(v.hours_total), 0)::DOUBLE PRECISION AS total_hours,
    COALESCE(SUM(v.miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(v.days), 0)::BIGINT AS total_days
FROM crew_assignments ca
JOIN voyages v ON v.id = ca.voyage_id
WHERE ca.crew_member_id = $1;

-- name: RepointCrewAssignmentsToVoyage :exec
UPDATE crew_assignments SET voyage_id = $1, trip_id = NULL
WHERE trip_id = $2;

-- name: AddAcceptedTripCrewAssignment :exec
-- Puts a user's person on a trip's crew as a załogant after their sign-up
-- for it was accepted. An existing crew entry for that person is kept as is,
-- so a hand-added entry stays hand-added.
INSERT INTO crew_assignments (trip_id, crew_member_id, added_by_acceptance)
SELECT sqlc.arg(trip_id)::BIGINT, cm.id, TRUE
FROM crew_members cm
WHERE cm.user_id = sqlc.arg(user_id)::BIGINT
ON CONFLICT (trip_id, crew_member_id) WHERE trip_id IS NOT NULL DO NOTHING;

-- name: RemoveAcceptedTripCrewAssignment :exec
-- Takes a user's person off a trip's crew when the entry came from an
-- accepted sign-up and the user no longer has any accepted sign-up for the
-- trip, direct or through a cruise enrollment assigned to it.
DELETE FROM crew_assignments ca
USING crew_members cm
WHERE ca.trip_id = sqlc.arg(trip_id)::BIGINT
  AND ca.added_by_acceptance
  AND cm.id = ca.crew_member_id
  AND cm.user_id = sqlc.arg(user_id)::BIGINT
  AND NOT EXISTS (
      SELECT 1 FROM trip_enrollments te
      WHERE te.trip_id = ca.trip_id AND te.user_id = cm.user_id AND te.status = 'accepted'
  )
  AND NOT EXISTS (
      SELECT 1 FROM cruise_enrollments ce
      WHERE ce.trip_id = ca.trip_id AND ce.user_id = cm.user_id AND ce.status = 'accepted'
  );

-- name: MergeSharedCrewAssignments :exec
-- On a leg or voyage both people are on, the kept person's assignment stays.
-- It takes the duplicate's patent number when it has none, and counts as
-- added by an accepted sign-up only when both entries did, so a hand-added
-- entry is never removed when the sign-up is withdrawn.
UPDATE crew_assignments k SET
    patent_number = COALESCE(k.patent_number, d.patent_number),
    added_by_acceptance = k.added_by_acceptance AND d.added_by_acceptance
FROM crew_assignments d
WHERE k.crew_member_id = sqlc.arg(kept_id)::BIGINT
  AND d.crew_member_id = sqlc.arg(duplicate_id)::BIGINT
  AND (k.trip_id = d.trip_id OR k.voyage_id = d.voyage_id);

-- name: DeleteSharedCrewAssignments :execrows
DELETE FROM crew_assignments d
USING crew_assignments k
WHERE d.crew_member_id = sqlc.arg(duplicate_id)::BIGINT
  AND k.crew_member_id = sqlc.arg(kept_id)::BIGINT
  AND (k.trip_id = d.trip_id OR k.voyage_id = d.voyage_id);

-- name: MoveCrewAssignments :execrows
UPDATE crew_assignments SET crew_member_id = sqlc.arg(kept_id)::BIGINT
WHERE crew_member_id = sqlc.arg(duplicate_id)::BIGINT;
