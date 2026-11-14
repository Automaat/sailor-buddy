# Image upload storage

Image uploads require an approved account and verified email, like other club
data routes. The API decodes JPEG, PNG and non-animated WebP before storing them.
The original encoded bytes are retained; image metadata is not stripped.

| Limit | Default | Configuration |
| --- | --- | --- |
| Encoded file | 5 MiB | Fixed |
| Entire multipart request | 5 MiB + 64 KiB | Fixed; exactly one `file` part |
| Width and height | 4096 pixels each | Fixed |
| Decoded area | 4,000,000 pixels | Fixed; checked before full decode |
| Stored images per account | 100 files | `SAILOR_UPLOAD_MAX_FILES` |
| Image storage per account | 100 MiB | `SAILOR_UPLOAD_MAX_BYTES`, in bytes |
| Upload/cleanup requests per account | 20 per minute | `SAILOR_UPLOAD_REQUESTS_PER_MINUTE` |
| Unattached upload eligibility for cleanup | 24 hours | Fixed |

Overrides must be positive integers. Maximum values are 100,000 files, 1 TiB and
1000 requests per minute. Empty, zero, negative, invalid and overflowing values
stop startup. The request limit uses one-minute windows starting at the first
request; invalid multipart bodies and rejected uploads consume a request. An
account's upload and cleanup endpoints share the same counter. Denials return
429 with `Retry-After: 60`; storage quota denials return 409. Invalid images
return 400, oversized request bodies 413, and extra multipart fields/parts 422.
Storage or database failures return 503.

## Accounting across API processes

Every API replica must use the same API version, PostgreSQL database, upload volume and limit
configuration. PostgreSQL session advisory locks serialize quota checks, writes
and cleanup per account. A connection is discarded if a canceled lock acquisition
or unlock has an uncertain outcome, so a pooled connection cannot keep the lock.
Database work and copy cancellation checks have a 20-second deadline; filesystem
calls still depend on the mounted storage responding.

Before a file is created, its full byte/file reservation is committed to
`image_uploads`. The reservation counts even if another replica has a stale or
missing filesystem view. Existing untracked files in `<account>/images` also
count toward that account's quota. Physical files larger than their reservation
are charged for the excess. Cleanup and failed-write removal release reservations
only after unlinking succeeds or the file is confirmed missing. These quotas
cover image uploads, including legacy images; generated opinion documents have
their separate record-based lifecycle.

The service creates files exclusively and confines access to opened filesystem
roots. File serving is confined to the caller's account directory; a symlink
inside uploads cannot expose another account's images. Mounts must be writable
by the API user and retain their contents across replica replacement. Confirm
the actual production volume, credentials, backups and consistency in #690.

## Attachment and cleanup rules

New uploads return a managed URL of the form
`/api/uploads/<account>/images/m-<uuid>.<extension>`. Only files registered by this
version of the service are eligible for automatic cleanup. Startup does not
import or delete legacy uploads.

Creating or updating a cruise, trip or voyage protects every managed image in
its logo, photo and route fields before writing the content record. The service
normalizes local paths, encoded paths, relative paths and absolute URLs for this
check. A concurrent cleanup either skips an attachment that won the race, or
marks the upload unavailable first and makes the save fail with 422. A save
cannot attach an image already claimed for deletion.

Protection is permanent and conservative: a failed content save or a later
replacement/deletion of the record does not unprotect its images. Those files
still count toward the uploader's quota. Removing previously attached images
requires an operator's reference review; this release does not automatically
delete them or increase the quota.

An account's next valid upload cleans up to 100 of its expired, never-protected
uploads before checking quota. `POST /api/upload/images/cleanup` performs the
same cleanup explicitly and returns 204. It takes no account ID or file path and
cannot clean another account's uploads. Idle accounts keep their files until
their next upload or cleanup call; their existing storage remains quota-bound.

Cleanup first commits a `deleting` marker, which prevents attachment, then
unlinks the file and removes its database record. Failed unlinks keep the
reservation and marker for retry. Missing files are safe to retry. Interrupted
writes are removed immediately when the handler knows it created the file;
process crashes after ownership confirmation are recovered by expired-upload
cleanup. If a crash occurs between exclusive creation and ownership confirmation,
an empty file can remain unconfirmed. Cleanup drops its reservation but preserves
the file for operator review; the file itself still counts against quota. This
avoids deleting an existing file after a creation collision or ambiguous outcome.

Do not manually remove live reservations or edit image URLs directly in SQL.
Restore the database and upload volume together: a missing tracked file remains
charged until eligible cleanup, while restored untracked files remain protected
and count as legacy storage.

## Verification

Handler tests use real PostgreSQL 18 and temporary upload directories. They cover
byte/file boundaries, eight competing OS processes, missing-file reservations,
rate-limit concurrency and reset, all nine image fields on create and update,
attachment/cleanup interleavings, legacy and other-owner protection, cleanup
retries, invalid content, WebP decoding, and write/close/cancellation failures.
Set `SAILOR_TEST_DATABASE_URL` to a disposable local PostgreSQL server, then run
`cd backend && mise exec -- go test ./internal/api/handlers`.
