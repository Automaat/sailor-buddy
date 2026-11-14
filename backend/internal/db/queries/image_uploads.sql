-- name: CountImageUploadRequest :one
INSERT INTO image_upload_requests (user_id, window_start, requests)
VALUES ($1, clock_timestamp(), 1)
ON CONFLICT (user_id) DO UPDATE SET
    window_start = CASE WHEN image_upload_requests.window_start <= EXCLUDED.window_start - interval '1 minute'
                        THEN EXCLUDED.window_start ELSE image_upload_requests.window_start END,
    requests = CASE WHEN image_upload_requests.window_start <= EXCLUDED.window_start - interval '1 minute'
                    THEN 1 ELSE image_upload_requests.requests + 1 END
WHERE image_upload_requests.window_start <= EXCLUDED.window_start - interval '1 minute'
      OR image_upload_requests.requests < sqlc.arg(request_limit)
RETURNING requests;

-- name: ReserveImageUpload :exec
INSERT INTO image_uploads (url, user_id, byte_size, state) VALUES ($1, $2, $3, 'reserved');

-- name: ListImageUploadsForQuota :many
SELECT url, byte_size FROM image_uploads WHERE user_id = $1 LIMIT sqlc.arg(row_limit)::bigint;

-- name: SetImageUploadState :exec
UPDATE image_uploads SET state = $2, owned = owned OR $2 IN ('writing', 'ready')
WHERE url = $1 AND NOT retained;

-- name: RetainImageUpload :one
UPDATE image_uploads SET retained = TRUE WHERE url = $1 AND state = 'ready' RETURNING url;

-- name: ClaimAbandonedImageUploads :many
UPDATE image_uploads SET state = 'deleting' WHERE url IN (
    SELECT candidate.url FROM image_uploads candidate WHERE candidate.user_id = $1 AND NOT candidate.retained
        AND candidate.created_at < clock_timestamp() - interval '24 hours'
    ORDER BY candidate.created_at LIMIT 100 FOR UPDATE SKIP LOCKED
) AND NOT retained RETURNING *;

-- name: DeleteAbandonedImageUpload :exec
DELETE FROM image_uploads WHERE url = $1 AND NOT retained;
