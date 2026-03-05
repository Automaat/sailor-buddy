-- name: CreateVoyageOpinion :one
INSERT INTO voyage_opinions (cruise_id, crew_member_id, file_path, file_format) VALUES (?, ?, ?, ?) RETURNING *;

-- name: ListCruiseVoyageOpinions :many
SELECT vo.*, cm.full_name
FROM voyage_opinions vo
JOIN crew_members cm ON cm.id = vo.crew_member_id
WHERE vo.cruise_id = ?
ORDER BY cm.full_name;

-- name: GetVoyageOpinion :one
SELECT * FROM voyage_opinions WHERE id = ?;

-- name: DeleteVoyageOpinion :exec
DELETE FROM voyage_opinions WHERE id = ?;
