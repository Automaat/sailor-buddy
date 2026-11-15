-- name: CreateYacht :one
INSERT INTO yachts (created_by, name, registration_no, yacht_type, berths, length_m, build_year, engine, owner_name)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: AddVoyageYacht :one
-- Adds a club yacht from a voyage form for an admin or the account linked
-- to the voyage's captain person, else inserts no row.
INSERT INTO yachts (created_by, name, registration_no, yacht_type, berths, length_m, build_year, engine, owner_name)
SELECT sqlc.arg(caller_id)::BIGINT, sqlc.arg(name)::TEXT, sqlc.narg(registration_no)::TEXT,
    sqlc.narg(yacht_type)::TEXT, sqlc.narg(berths)::BIGINT, sqlc.narg(length_m)::DOUBLE PRECISION,
    sqlc.narg(build_year)::BIGINT, sqlc.narg(engine)::TEXT, sqlc.narg(owner_name)::TEXT
WHERE sqlc.arg(caller_is_admin)::BOOLEAN OR EXISTS (
    SELECT 1 FROM voyages v JOIN crew_members cm ON cm.id = v.captain_person_id
    WHERE v.id = sqlc.arg(voyage_id)::BIGINT AND cm.user_id = sqlc.arg(caller_id)::BIGINT)
RETURNING *;

-- name: GetYacht :one
SELECT * FROM yachts WHERE id = $1;

-- name: ListYachts :many
-- A set after_id starts the page after the cursor's (name, id) sort key.
SELECT * FROM yachts
WHERE sqlc.narg(after_id)::BIGINT IS NULL
   OR (name, id) > (sqlc.narg(after_name)::TEXT, sqlc.narg(after_id)::BIGINT)
ORDER BY name, id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountYachts :one
SELECT COUNT(*)::BIGINT FROM yachts;

-- name: UpdateYacht :exec
UPDATE yachts SET name = $1, registration_no = $2, yacht_type = $3, berths = $4,
    length_m = $5, build_year = $6, engine = $7, owner_name = $8, updated_at = CURRENT_TIMESTAMP
WHERE id = $9;

-- name: DeleteYacht :exec
DELETE FROM yachts WHERE id = $1;
