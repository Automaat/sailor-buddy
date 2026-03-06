-- name: CreateCrewMember :one
INSERT INTO crew_members (owner_id, user_id, full_name, email, patent_number) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetCrewMember :one
SELECT * FROM crew_members WHERE id = $1 AND owner_id = $2;

-- name: ListCrewMembers :many
SELECT * FROM crew_members WHERE owner_id = $1 ORDER BY full_name;

-- name: UpdateCrewMember :exec
UPDATE crew_members SET full_name = $1, email = $2, patent_number = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4 AND owner_id = $5;

-- name: DeleteCrewMember :exec
DELETE FROM crew_members WHERE id = $1 AND owner_id = $2;

-- name: GetCrewMemberByName :one
SELECT * FROM crew_members WHERE owner_id = $1 AND full_name = $2;
