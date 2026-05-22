-- name: CreateCrewMember :one
INSERT INTO crew_members (
    created_by, user_id, full_name, email, patent_number,
    phone, pzz_license_type, pzz_license_number,
    emergency_contact_name, emergency_contact_phone
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING *;

-- name: GetCrewMember :one
SELECT * FROM crew_members WHERE id = $1;

-- name: ListCrewMembers :many
SELECT * FROM crew_members ORDER BY full_name, id LIMIT $1 OFFSET $2;

-- name: CountCrewMembers :one
SELECT COUNT(*)::BIGINT FROM crew_members;

-- name: UpdateCrewMember :exec
UPDATE crew_members SET
    full_name = $1, email = $2, patent_number = $3,
    phone = $4, pzz_license_type = $5, pzz_license_number = $6,
    emergency_contact_name = $7, emergency_contact_phone = $8,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $9;

-- name: DeleteCrewMember :exec
DELETE FROM crew_members WHERE id = $1;

-- name: GetCrewMemberByName :one
SELECT * FROM crew_members WHERE full_name = $1;
