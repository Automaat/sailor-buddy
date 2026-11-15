-- name: CreateVoyageOpinion :one
INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path, file_format) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpsertVoyageOpinion :one
-- Stores the opinion for an admin or the account linked to the voyage's
-- captain person, else writes no row. replaced_path is the file of the
-- opinion it replaced, empty for a new one.
INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path, file_format)
SELECT sqlc.arg(voyage_id)::BIGINT, sqlc.arg(crew_member_id)::BIGINT, sqlc.arg(file_path)::TEXT,
    sqlc.arg(file_format)::TEXT
WHERE sqlc.arg(caller_is_admin)::BOOLEAN OR EXISTS (
    SELECT 1 FROM voyages v JOIN crew_members cm ON cm.id = v.captain_person_id
    WHERE v.id = sqlc.arg(voyage_id)::BIGINT AND cm.user_id = sqlc.arg(caller_id)::BIGINT)
ON CONFLICT (voyage_id, crew_member_id) DO UPDATE
SET file_path = EXCLUDED.file_path, file_format = EXCLUDED.file_format, created_at = CURRENT_TIMESTAMP
RETURNING id, voyage_id, crew_member_id, file_path, file_format, created_at,
    COALESCE(old.file_path, '')::TEXT AS replaced_path;

-- name: ListVoyageVoyageOpinions :many
SELECT vo.*, cm.full_name
FROM voyage_opinions vo
JOIN crew_members cm ON cm.id = vo.crew_member_id
WHERE vo.voyage_id = $1
ORDER BY cm.full_name;

-- name: GetVoyageOpinion :one
SELECT * FROM voyage_opinions WHERE id = $1;

-- name: DeleteVoyageOpinion :one
-- Deletes the voyage's opinion for an admin or the account linked to the
-- voyage's captain person. It answers whether the caller passed that check
-- and the deleted opinion's file, empty when no opinion matched.
WITH allowed AS (
    SELECT (sqlc.arg(caller_is_admin)::BOOLEAN OR EXISTS (
        SELECT 1 FROM voyages v JOIN crew_members cm ON cm.id = v.captain_person_id
        WHERE v.id = sqlc.arg(voyage_id)::BIGINT AND cm.user_id = sqlc.arg(caller_id)::BIGINT)) AS ok
), deleted AS (
    DELETE FROM voyage_opinions o
    WHERE o.id = sqlc.arg(id)::BIGINT AND o.voyage_id = sqlc.arg(voyage_id)::BIGINT
      AND (SELECT ok FROM allowed)
    RETURNING o.file_path
)
SELECT allowed.ok::BOOLEAN AS ok, COALESCE((SELECT file_path FROM deleted), '')::TEXT AS file_path FROM allowed;

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
