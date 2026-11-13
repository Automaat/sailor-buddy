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

