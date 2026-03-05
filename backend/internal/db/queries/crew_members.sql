-- name: CreateCrewMember :one
INSERT INTO crew_members (owner_id, user_id, full_name, email, patent_number) VALUES (?, ?, ?, ?, ?) RETURNING *;

-- name: GetCrewMember :one
SELECT * FROM crew_members WHERE id = ? AND owner_id = ?;

-- name: ListCrewMembers :many
SELECT * FROM crew_members WHERE owner_id = ? ORDER BY full_name;

-- name: UpdateCrewMember :exec
UPDATE crew_members SET full_name = ?, email = ?, patent_number = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND owner_id = ?;

-- name: DeleteCrewMember :exec
DELETE FROM crew_members WHERE id = ? AND owner_id = ?;

-- name: GetCrewMemberByName :one
SELECT * FROM crew_members WHERE owner_id = ? AND full_name = ?;
