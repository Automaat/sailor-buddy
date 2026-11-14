# Security logs

The API writes JSON request and audit records to stderr. The hosting platform
must collect and protect that stream; the app has no public log endpoint.

## Fields and scope

Every request gets a fresh server-generated `X-Request-ID`. The same ID appears
in the `http_request` event and, for mutations or HTTP errors, a
`security_audit` event. CORS exposes the header to the configured frontend.
Client-supplied request IDs are ignored.

| Field | Meaning |
| --- | --- |
| `actor_id` | Verified database account ID; `0` when identity is unknown or anonymous |
| `action` | Operation ID from the registered API contract |
| `route` | Registered route template, e.g. `/api/enroll/{token}`; `unmatched` for unknown paths |
| `targets` | Positive numeric path IDs, plus account/person IDs supplied by person-link/merge handlers |
| `result` | `success` for 2xx, `denied` for 401/403, `rejected` for other non-2xx below 500, `failure` for 5xx/panic |
| `status` | HTTP status committed by the app; `0` for an aborted request without a committed response |
| `duration_ms` | Request duration, on `http_request` records |

The action and path targets are identified before authentication, so rejected
role, access, payment, enrollment, link, and deletion requests remain attributable
to their intended operation. The actor comes only from the verified account,
never from a target user ID. Collection creates can have no record ID yet;
their route identifies the target collection. A successful GET produces a request
record with actor/action/targets but no separate mutation event.

For example, an admin changing another account's role produces these payload
fields (alongside the normal slog timestamp, level, and message):

```json
{
  "event": "security_audit",
  "request_id": "d11b0547-20d0-4b09-b692-a751475de283",
  "actor_id": 12,
  "method": "PUT",
  "route": "/api/members/{userID}/role",
  "action": "update-member-role",
  "targets": {"user_id": 34},
  "result": "success",
  "status": 204
}
```

Records describe the HTTP outcome, not a transaction ledger or proof that the
client received the response. A failure after a database commit does not undo
that commit. An interrupted response that already committed 200 retains status
200 and records `result: failure` when a panic is caught.

## Redaction

Request/audit records contain no raw URL, query string, headers, request or
response body, enrollment token, password, emergency contact, opinion text, or
client IP. Unknown paths and unsupported HTTP methods never become log text.
Uploaded file paths are replaced by the fixed `/api/uploads/*` template.
Panic values and stacks are not printed by the request recovery path.

The API's diagnostic JSON handler drops personal text attributes (`email`,
`uid`, `name`, `query`, `path`) and credential/body attributes. Errors retain
only a type/category or PostgreSQL SQLSTATE, never an error message or detail
that can contain submitted values. Startup/operator command output is separate
from the request/audit stream and must also remain private.

## Collection and failures

- Collect stderr over the hosting platform's protected log channel. Restrict
  access to designated operators; ordinary club accounts must not read it.
- Use a 90-day retention window for request/audit logs, then delete them.
  Apply the same limit to exports and backups. Change this policy explicitly
  when the club chooses a different retention requirement.
- Alert on collection gaps, storage exhaustion, and `security_log_write_failed`.
  Logging is synchronous and has no application queue; a blocked writer adds
  request latency. Bound the platform log driver's buffering and backpressure.
- Writes are best effort. If the logger returns an error, the app attempts a
  fixed `security_log_write_failed` notice on stderr and preserves the HTTP
  result. It cannot roll back a completed mutation or guarantee delivery when
  stderr or the collector fails. Recover the logging destination and investigate
  the affected interval; do not assume missing events mean no action occurred.
- Configure ingress, CDN, and frontend platform logs separately: disable body
  and credential logging, redact enrollment path segments and query strings,
  and prevent raw URL dumps. The API cannot sanitize a proxy's own logs.

Deployment verification #690 must confirm collection, restricted access,
retention, failure alerts, and proxy redaction on the chosen host. None of
those hosting controls has been verified before deployment.
