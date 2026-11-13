-- name: CreateCrewMember :one
INSERT INTO crew_members (
    created_by, user_id, full_name, email, patent_number,
    phone, pzz_license_type, pzz_license_number,
    emergency_contact_name, emergency_contact_phone, other_names
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE(sqlc.arg(other_names)::JSONB, '[]'::jsonb)) RETURNING *;

-- name: GetCrewMember :one
SELECT * FROM crew_members WHERE id = $1;

-- name: ListCrewMembers :many
-- A NULL search lists everyone; otherwise it matches a case-insensitive
-- substring of the name, any other name, or the e-mail.
SELECT * FROM crew_members
WHERE sqlc.narg(search)::TEXT IS NULL
   OR strpos(lower(full_name), lower(sqlc.narg(search)::TEXT)) > 0
   OR strpos(lower(COALESCE(email, '')), lower(sqlc.narg(search)::TEXT)) > 0
   OR EXISTS (
       SELECT 1 FROM jsonb_array_elements_text(other_names) AS other(name)
       WHERE strpos(lower(other.name), lower(sqlc.narg(search)::TEXT)) > 0
   )
ORDER BY full_name, id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountCrewMembers :one
SELECT COUNT(*)::BIGINT FROM crew_members
WHERE sqlc.narg(search)::TEXT IS NULL
   OR strpos(lower(full_name), lower(sqlc.narg(search)::TEXT)) > 0
   OR strpos(lower(COALESCE(email, '')), lower(sqlc.narg(search)::TEXT)) > 0
   OR EXISTS (
       SELECT 1 FROM jsonb_array_elements_text(other_names) AS other(name)
       WHERE strpos(lower(other.name), lower(sqlc.narg(search)::TEXT)) > 0
   );

-- name: EnsureUserPerson :exec
-- Gives an account its person on first sign-in. The NOT EXISTS guard keeps
-- repeat sign-ins read-only; ON CONFLICT covers two concurrent first requests.
INSERT INTO crew_members (user_id, full_name, email, phone, patent_number)
SELECT u.id, COALESCE(NULLIF(btrim(u.name), ''), u.email), u.email, u.phone, u.patent_number
FROM users u
WHERE u.id = $1
  AND NOT EXISTS (SELECT 1 FROM crew_members cm WHERE cm.user_id = u.id)
ON CONFLICT (user_id) DO NOTHING;

-- name: GetCrewMemberByUserID :one
SELECT * FROM crew_members WHERE user_id = $1;

-- name: UpdateCrewMember :exec
UPDATE crew_members SET
    full_name = $1, email = $2, patent_number = $3,
    phone = $4, pzz_license_type = $5, pzz_license_number = $6,
    emergency_contact_name = $7, emergency_contact_phone = $8,
    other_names = $9, updated_at = CURRENT_TIMESTAMP
WHERE id = $10;

-- name: DeleteUnlinkedCrewMember :execrows
-- Deletes a person without an account. A person linked to an account is
-- kept, so every account keeps exactly one person.
DELETE FROM crew_members WHERE id = $1 AND user_id IS NULL;


-- name: LockCrewMembersForMerge :many
-- Locks both people in id order, so two merges cannot deadlock and no crew
-- assignment or opinion can be added for either until the merge ends.
SELECT * FROM crew_members
WHERE id IN (sqlc.arg(first_id)::BIGINT, sqlc.arg(second_id)::BIGINT)
ORDER BY id
FOR UPDATE;

-- name: CountCrewMergeRows :one
-- Counts what merging the duplicate into the kept person moves. A shared
-- row is a leg, voyage or voyage opinion both people already have.
SELECT
    (SELECT COUNT(*) FROM crew_assignments
     WHERE crew_member_id = sqlc.arg(duplicate_id)::BIGINT AND trip_id IS NOT NULL)::BIGINT AS trip_assignments,
    (SELECT COUNT(*) FROM crew_assignments
     WHERE crew_member_id = sqlc.arg(duplicate_id)::BIGINT AND voyage_id IS NOT NULL)::BIGINT AS voyage_assignments,
    (SELECT COUNT(*) FROM crew_assignments d
     JOIN crew_assignments k ON k.crew_member_id = sqlc.arg(kept_id)::BIGINT
      AND (k.trip_id = d.trip_id OR k.voyage_id = d.voyage_id)
     WHERE d.crew_member_id = sqlc.arg(duplicate_id)::BIGINT)::BIGINT AS shared_assignments,
    (SELECT COUNT(*) FROM voyage_opinions
     WHERE crew_member_id = sqlc.arg(duplicate_id)::BIGINT)::BIGINT AS opinions,
    (SELECT COUNT(*) FROM voyage_opinions d
     JOIN voyage_opinions k ON k.crew_member_id = sqlc.arg(kept_id)::BIGINT AND k.voyage_id = d.voyage_id
     WHERE d.crew_member_id = sqlc.arg(duplicate_id)::BIGINT)::BIGINT AS shared_opinions;

-- name: DeleteMergedCrewMember :execrows
DELETE FROM crew_members WHERE id = $1;

-- name: UpdateMergedCrewMember :one
UPDATE crew_members SET
    user_id = $1, full_name = $2, email = $3, patent_number = $4, phone = $5,
    pzz_license_type = $6, pzz_license_number = $7,
    emergency_contact_name = $8, emergency_contact_phone = $9,
    other_names = $10, updated_at = CURRENT_TIMESTAMP
WHERE id = $11
RETURNING *;

-- name: LockUserForUpdate :one
-- Serializes a profile save with a merge that moves the account's person
-- link. Both lock the account first; the save's update then runs on a fresh
-- snapshot and finds the person the merge kept.
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: LockCrewMergeTrips :exec
-- Share-locks every leg either person is on, so completing a leg waits for
-- the merge instead of repointing its crew under it.
SELECT t.id FROM trips t
WHERE t.id IN (
    SELECT ca.trip_id FROM crew_assignments ca
    WHERE ca.crew_member_id IN (sqlc.arg(first_id)::BIGINT, sqlc.arg(second_id)::BIGINT)
)
ORDER BY t.id
FOR SHARE;

-- name: LockCrewMergeVoyages :exec
-- Share-locks every voyage either person is on, for the same reason.
SELECT v.id FROM voyages v
WHERE v.id IN (
    SELECT ca.voyage_id FROM crew_assignments ca
    WHERE ca.crew_member_id IN (sqlc.arg(first_id)::BIGINT, sqlc.arg(second_id)::BIGINT)
)
ORDER BY v.id
FOR SHARE;
