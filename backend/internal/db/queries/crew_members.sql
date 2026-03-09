-- name: CreateCrewMember :one
INSERT INTO crew_members (owner_id, user_id, full_name, email, patent_number) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetCrewMember :one
SELECT * FROM crew_members WHERE id = $1 AND owner_id = $2 AND org_id IS NULL;

-- name: ListCrewMembers :many
SELECT * FROM crew_members WHERE owner_id = $1 AND org_id IS NULL ORDER BY full_name;

-- name: UpdateCrewMember :exec
UPDATE crew_members SET full_name = $1, email = $2, patent_number = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4 AND owner_id = $5 AND org_id IS NULL;

-- name: DeleteCrewMember :exec
DELETE FROM crew_members WHERE id = $1 AND owner_id = $2 AND org_id IS NULL;

-- name: GetCrewMemberByName :one
SELECT * FROM crew_members WHERE owner_id = $1 AND full_name = $2 AND org_id IS NULL;

-- name: CreateOrgCrewMember :one
INSERT INTO crew_members (owner_id, org_id, user_id, full_name, email, patent_number, phone, pzz_license_type, pzz_license_number, emergency_contact_name, emergency_contact_phone)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING *;

-- name: GetOrgCrewMember :one
SELECT * FROM crew_members WHERE id = $1 AND org_id = $2;

-- name: ListOrgCrewMembers :many
SELECT * FROM crew_members WHERE org_id = $1 ORDER BY full_name;

-- name: UpdateOrgCrewMember :exec
UPDATE crew_members SET
    full_name = $1, email = $2, patent_number = $3,
    phone = $4, pzz_license_type = $5, pzz_license_number = $6,
    emergency_contact_name = $7, emergency_contact_phone = $8,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $9 AND org_id = $10;

-- name: DeleteOrgCrewMember :exec
DELETE FROM crew_members WHERE id = $1 AND org_id = $2;
