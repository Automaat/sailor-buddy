-- name: ListPersonClaimCandidates :many
-- People without an account that the user may claim. A NULL search lists
-- only the people whose e-mail matches the account's; otherwise it matches
-- a case-insensitive part of the name or any other name. E-mail matches
-- come first.
SELECT cm.id, cm.full_name, cm.other_names,
       (cm.email IS NOT NULL AND lower(btrim(cm.email)) = lower(btrim(u.email)))::BOOLEAN AS email_match
FROM crew_members cm, users u
WHERE u.id = sqlc.arg(user_id)
  AND cm.user_id IS NULL
  AND (
      (sqlc.narg(search)::TEXT IS NULL
       AND cm.email IS NOT NULL AND lower(btrim(cm.email)) = lower(btrim(u.email)))
   OR strpos(lower(cm.full_name), lower(sqlc.narg(search)::TEXT)) > 0
   OR EXISTS (
       SELECT 1 FROM jsonb_array_elements_text(cm.other_names) AS other(name)
       WHERE strpos(lower(other.name), lower(sqlc.narg(search)::TEXT)) > 0
   )
  )
ORDER BY email_match DESC, cm.full_name, cm.id
LIMIT 20;

-- name: CreatePersonLinkRequest :one
INSERT INTO person_link_requests (user_id, person_id, person_name) VALUES ($1, $2, $3) RETURNING *;

-- name: GetLatestPersonLinkRequest :one
-- The user's most recent request, so a rejection stays visible until the
-- user sends a new one. person_linked tells whether the requested person
-- still carries the account, which an admin's unlink ends.
SELECT r.*, (cm.user_id IS NOT DISTINCT FROM r.user_id)::BOOLEAN AS person_linked
FROM person_link_requests r
JOIN crew_members cm ON cm.id = r.person_id
WHERE r.user_id = $1
ORDER BY r.id DESC
LIMIT 1;

-- name: CancelPendingPersonLinkRequest :execrows
UPDATE person_link_requests SET status = 'cancelled', decided_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND status = 'pending';

-- name: ListPendingPersonLinkRequests :many
SELECT r.*, u.name AS user_name, u.email AS user_email,
       cm.full_name AS current_person_name, cm.email AS person_email
FROM person_link_requests r
JOIN users u ON u.id = r.user_id
JOIN crew_members cm ON cm.id = r.person_id
WHERE r.status = 'pending'
ORDER BY r.created_at, r.id;

-- name: CountPendingPersonLinkRequests :one
SELECT COUNT(*)::BIGINT FROM person_link_requests WHERE status = 'pending';

-- name: LockPersonLinkRequest :one
-- Locks the request, so a cancel or a second decision waits for this one
-- and then finds it no longer pending.
SELECT * FROM person_link_requests WHERE id = $1 FOR UPDATE;

-- name: DecidePersonLinkRequest :exec
UPDATE person_link_requests SET
    status = sqlc.arg(status), decided_by = sqlc.arg(decided_by), decided_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: CloseUserPendingLinkRequests :exec
-- After an admin links the account to a person directly, its pending
-- request is approved when it asked for that person and rejected otherwise.
UPDATE person_link_requests SET
    status = CASE WHEN person_id = sqlc.arg(person_id)::BIGINT THEN 'approved' ELSE 'rejected' END,
    decided_by = sqlc.arg(decided_by), decided_at = CURRENT_TIMESTAMP
WHERE user_id = sqlc.arg(user_id) AND status = 'pending';

-- name: UnlinkCrewMember :execrows
-- Detaches the person from its account. The e-mail goes too when it is the
-- account's, since an unlinked person with the account's e-mail would still
-- count toward the account's cruises.
UPDATE crew_members SET
    user_id = NULL,
    email = CASE WHEN email IS NOT NULL AND lower(btrim(email)) = lower(btrim(sqlc.arg(account_email)::TEXT))
        THEN NULL ELSE email END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: MovePersonLinkRequests :execrows
-- Points the duplicate's requests at the kept person when two people are
-- merged, so a pending request survives the duplicate's deletion.
UPDATE person_link_requests SET person_id = sqlc.arg(kept_id)::BIGINT
WHERE person_id = sqlc.arg(duplicate_id)::BIGINT;

-- name: LockCrewMemberForShare :one
-- Holds the requested person while a request is created, so a concurrent
-- link or merge either finishes first and is seen, or waits for the request.
SELECT * FROM crew_members WHERE id = $1 FOR SHARE;
