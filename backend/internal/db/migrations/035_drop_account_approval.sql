-- A verified email is enough to use the app: an account no longer waits for
-- an administrator to approve it, so users.access_approved goes. Suspension
-- stays the club's way to deny an account, so users.suspended is kept.

ALTER TABLE users DROP COLUMN access_approved;
