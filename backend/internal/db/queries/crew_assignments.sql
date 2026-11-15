-- name: CreateVoyageCrewAssignment :one
INSERT INTO crew_assignments (voyage_id, crew_member_id, role, patent_number)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListTripCrewAssignments :many
-- A leg's crew with the people's phone and emergency contact.
-- contacts_visible is true only for an admin or the caller who captains this
-- leg, and the handler drops the contacts otherwise. The captain check runs
-- in the same statement as the crew read, so a captain change committed
-- meanwhile can never pair the former captain with a newer member's contacts.
SELECT ca.id, ca.trip_id, ca.voyage_id, ca.crew_member_id, ca.role, ca.patent_number, ca.created_at,
       cm.full_name, cm.email, cm.phone, cm.emergency_contact_name, cm.emergency_contact_phone,
       access.contacts_visible
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
CROSS JOIN (
    SELECT (sqlc.arg(is_admin)::BOOLEAN OR EXISTS (
        SELECT 1 FROM trips t
        JOIN crew_members cap ON cap.id = t.captain_person_id
        WHERE t.id = sqlc.arg(trip_id)::BIGINT
          AND cap.user_id = sqlc.narg(caller_user_id)::BIGINT
    ))::BOOLEAN AS contacts_visible
) access
WHERE ca.trip_id = sqlc.arg(trip_id)::BIGINT
ORDER BY cm.full_name;

-- name: ListVoyageCrewAssignments :many
SELECT ca.id, ca.trip_id, ca.voyage_id, ca.crew_member_id, ca.role, ca.patent_number, ca.created_at,
       cm.full_name, cm.email
FROM crew_assignments ca
JOIN crew_members cm ON cm.id = ca.crew_member_id
WHERE ca.voyage_id = $1
ORDER BY cm.full_name;

-- name: UpdateTripCrewAssignmentRole :one
-- Sets the role of one crew entry of a trip. No row when the trip has no
-- such entry.
UPDATE crew_assignments ca SET role = sqlc.arg(role)
FROM crew_members cm
WHERE ca.id = sqlc.arg(id) AND ca.trip_id = sqlc.arg(trip_id)::BIGINT AND cm.id = ca.crew_member_id
RETURNING ca.id, ca.trip_id, ca.voyage_id, ca.crew_member_id, ca.role, ca.patent_number, ca.created_at,
          cm.full_name, cm.email, cm.phone, cm.emergency_contact_name, cm.emergency_contact_phone;

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
-- Puts an accepted sign-up's person on a trip's crew as a załogant. An
-- existing crew entry for that person is kept as is, so a hand-added entry
-- stays hand-added.
INSERT INTO crew_assignments (trip_id, crew_member_id, added_by_acceptance)
VALUES (sqlc.arg(trip_id)::BIGINT, sqlc.arg(crew_member_id)::BIGINT, TRUE)
ON CONFLICT (trip_id, crew_member_id) WHERE trip_id IS NOT NULL DO NOTHING;

-- name: RemoveAcceptedTripCrewAssignment :exec
-- Takes a person off a trip's crew when the entry came from an accepted
-- sign-up and the person no longer has any accepted sign-up for the trip:
-- a direct one, or a cruise enrollment of the person's account assigned to
-- it. The person is crew_member_id, or the person of the account user_id.
DELETE FROM crew_assignments ca
USING crew_members cm
WHERE ca.trip_id = sqlc.arg(trip_id)::BIGINT
  AND ca.added_by_acceptance
  AND cm.id = ca.crew_member_id
  AND (cm.id = sqlc.narg(crew_member_id)::BIGINT OR cm.user_id = sqlc.narg(user_id)::BIGINT)
  AND NOT EXISTS (
      SELECT 1 FROM trip_enrollments te
      WHERE te.trip_id = ca.trip_id AND te.crew_member_id = cm.id AND te.status = 'accepted'
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
