-- name: CreateVoyageOpinion :one
INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path, file_format) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpsertVoyageOpinion :one
INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path, file_format)
VALUES ($1, $2, $3, $4)
ON CONFLICT (voyage_id, crew_member_id) DO UPDATE
SET file_path = EXCLUDED.file_path, file_format = EXCLUDED.file_format, created_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: ListVoyageVoyageOpinions :many
SELECT vo.*, cm.full_name
FROM voyage_opinions vo
JOIN crew_members cm ON cm.id = vo.crew_member_id
WHERE vo.voyage_id = $1
ORDER BY cm.full_name;

-- name: GetVoyageOpinion :one
SELECT * FROM voyage_opinions WHERE id = $1;

-- name: DeleteVoyageOpinion :exec
DELETE FROM voyage_opinions WHERE id = $1;

-- name: DeleteSharedVoyageOpinions :many
-- On a voyage both people have an opinion for, the kept person's stays. The
-- returned file paths are removed once the merge commits.
DELETE FROM voyage_opinions d
USING voyage_opinions k
WHERE d.crew_member_id = sqlc.arg(duplicate_id)::BIGINT
  AND k.crew_member_id = sqlc.arg(kept_id)::BIGINT
  AND k.voyage_id = d.voyage_id
RETURNING d.file_path;

-- name: MoveVoyageOpinions :execrows
UPDATE voyage_opinions SET crew_member_id = sqlc.arg(kept_id)::BIGINT
WHERE crew_member_id = sqlc.arg(duplicate_id)::BIGINT;
