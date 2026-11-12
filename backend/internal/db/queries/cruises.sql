-- name: CreateCruise :one
INSERT INTO cruises (
    created_by, name, embark_date, disembark_date, countries, start_port, end_port,
    description, image_logo_url, image_photo_url, image_route_url,
    max_crew, cost_per_person, kind, series_id, edition,
    currency, cost_per_person_non_member, yacht_fund, deposit, included, not_included
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21, $22
) RETURNING *;

-- name: GetCruise :one
SELECT * FROM cruises WHERE id = $1;

-- name: ListCruises :many
SELECT * FROM cruises ORDER BY embark_date DESC NULLS LAST, id DESC LIMIT $1 OFFSET $2;

-- name: CountCruises :one
SELECT COUNT(*)::BIGINT FROM cruises;

-- name: UpdateCruise :exec
UPDATE cruises SET
    name = $1, embark_date = $2, disembark_date = $3, countries = $4,
    start_port = $5, end_port = $6, description = $7,
    image_logo_url = $8, image_photo_url = $9, image_route_url = $10,
    max_crew = $11, cost_per_person = $12, kind = $13,
    series_id = $14, edition = $15,
    currency = $16, cost_per_person_non_member = $17, yacht_fund = $18,
    deposit = $19, included = $20, not_included = $21,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $22;

-- name: DeleteCruise :exec
DELETE FROM cruises WHERE id = $1;

-- name: SetCruiseEnrollToken :exec
UPDATE cruises SET enroll_token = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2;

-- name: ClearCruiseEnrollToken :exec
UPDATE cruises SET enroll_token = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: GetCruiseByEnrollToken :one
SELECT id, name, embark_date, disembark_date, countries, start_port, end_port,
       description, image_photo_url, max_crew, cost_per_person,
       currency, cost_per_person_non_member, yacht_fund, deposit, included, not_included
FROM cruises WHERE enroll_token = $1;

-- name: ListCruiseTrips :many
SELECT * FROM trips WHERE cruise_id = $1 ORDER BY embark_date ASC, id ASC;

-- name: ListCruiseVoyages :many
SELECT * FROM voyages WHERE cruise_id = $1 ORDER BY year DESC, embark_date DESC, id DESC;
