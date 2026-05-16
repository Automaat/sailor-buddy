# Sailor Buddy API

HTTP/JSON API for the Sailor Buddy backend. All application endpoints live
under `/api`. The frontend dev server proxies `/api` to the backend on
`:8080` (see `frontend/vite.config.ts`).

## Authentication

Every `/api/*` endpoint requires a Firebase ID token:

```
Authorization: Bearer <firebase_id_token>
```

The `Auth` middleware verifies the token, then upserts a `users` row keyed by
Firebase UID (linking by verified email if the UID is new). The resulting user
identity scopes all owner-scoped data.

Public endpoints (no token):

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness probe, returns `200` with empty body |

There is no `/auth/register` endpoint — account provisioning happens
automatically on the first authenticated request.

### Auth failures

| Status | `error` value | Cause |
|--------|---------------|-------|
| 401 | `missing authorization header` | No `Authorization` header |
| 401 | `invalid authorization format` | Header not `Bearer <token>` |
| 401 | `invalid or expired token` | Firebase rejected the token |
| 401 | `missing email claim` | Token has no email claim |
| 401 | `email not verified` | Email-link collision, email unverified |
| 500 | `failed to provision user` | DB error during user upsert |

## Error responses

Errors use huma's RFC 9457 problem-details envelope:

```json
{
  "title": "Not Found",
  "status": 404,
  "detail": "trip not found"
}
```

Validation failures (`422`) additionally carry a per-field `errors` array:

```json
{
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "validation failed",
  "errors": [
    { "location": "body.name", "message": "expected string" }
  ]
}
```

Clients should branch on the HTTP status code (or `status`), not on the
`detail` string. The auth middleware still emits a legacy `{"error": "..."}`
body for `401` responses via `http.Error`.

## Status code conventions

| Status | Meaning | Example |
|--------|---------|---------|
| 200 | Success with body | `GET /api/trips`, `PUT`/`DELETE` on org invite accept |
| 201 | Resource created | `POST /api/trips`, `POST /api/orgs` |
| 204 | Success, no body | `PUT`/`DELETE` on most resources |
| 400 | Bad request — malformed body, failed validation, or unparseable path id | Missing `name`, `invalid trip id` |
| 401 | Unauthorized — missing/invalid token | Missing `Authorization` header |
| 403 | Forbidden — authenticated but not allowed | Non-member or non-admin on an org route |
| 404 | Not found — resource missing or not owned by caller | `GET /api/trips/999` |
| 409 | Conflict — uniqueness or state violation | Org slug taken, already enrolled |
| 410 | Gone — resource expired | Org invite expired or max uses reached |
| 500 | Internal server error | Database unavailable |

Notes:

- The whole API is served by the huma framework and described by
  `backend/openapi.yaml`. Malformed request bodies and unparseable path ids
  return `422` with a body-level error list. Some semantic checks still return
  `422` from the handler (`slug is required`, `cannot demote the last admin`).
- The error envelope is huma's RFC 9457 problem shape (`title`, `status`,
  `detail`, `errors`), not the legacy `{"error": "..."}` object.
- **Ownership is enforced via the query, not a separate check.** Requesting a
  resource owned by another user returns `404` (`sql.ErrNoRows`), not `403`.
- **Empty list responses serialize as `[]`**, never `null`.

## Pagination

Not implemented. List endpoints return the full result set as a JSON array.

## Data model & scoping

Two scopes exist:

- **Owner-scoped** (`/api/trips`, `/api/voyages`, `/api/yachts`, `/api/crew`,
  `/api/trainings`, `/api/dashboard`): rows filtered by the caller's
  `owner_id`. Other users' rows are invisible (`404` on direct access).
- **Org-scoped** (`/api/orgs/{slug}/...`): rows filtered by `org_id`. Access
  requires org membership; mutations require the `admin` role.

`crew_members` are decoupled from `users` — crew need not have accounts.

Trip lifecycle: a `trip` is `planned`, then either `cancelled` or `completed`.
Completing a trip atomically converts it into a `voyage` (a logged, finished
sailing record) and repoints crew assignments.

## Endpoint reference

`{id}` path segments are integers; `{slug}` and `{token}` are strings.
Mutating org routes additionally require the caller to be an org `admin`
(marked **admin** below).

### Identity

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Current user: `{id, email, name, avatar_url}` |
| GET | `/api/dashboard` | Owner sailing stats + per-year breakdown |

### Trips (owner-scoped)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/trips` | List trips | 200 |
| POST | `/api/trips` | Create trip | 201 |
| GET | `/api/trips/{id}` | Get trip | 200 |
| PUT | `/api/trips/{id}` | Update trip | 204 |
| DELETE | `/api/trips/{id}` | Delete trip | 204 |
| POST | `/api/trips/{id}/complete` | Complete trip → create voyage | 201 (voyage) |
| POST | `/api/trips/{id}/cancel` | Cancel trip | 200 (trip) |

`POST`/`PUT` body — only `name` is required:

```json
{
  "name": "Adriatic Week",
  "embark_date": "2026-06-01",
  "disembark_date": "2026-06-08",
  "countries": "Croatia",
  "start_port": "Split",
  "end_port": "Split",
  "captain_name": "Anna Kowalska",
  "yacht_id": 3,
  "cost_total": 4200.0,
  "cost_per_person": 600.0,
  "max_crew": 7,
  "image_logo_url": null,
  "image_photo_url": null,
  "image_route_url": null,
  "description": null,
  "cruise_id": null
}
```

`complete` body (all optional; `year` falls back to the embark date):

```json
{
  "year": 2026, "hours_total": 56, "hours_sail": 40, "hours_engine": 16,
  "hours_over_6bf": 4, "miles": 320, "days": 7, "tidal_waters": 1
}
```

`complete` returns `404` `trip not found or not in planned state` when the trip
is missing or already completed/cancelled. `cancel` returns `404`
`trip not found or invalid transition` likewise.

### Trip crew assignments

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/trips/{tripID}/crew` | List trip crew | 200 |
| POST | `/api/trips/{tripID}/crew` | Assign crew member | 201 |
| DELETE | `/api/trips/{tripID}/crew/{assignmentID}` | Remove assignment | 204 |

Assignment body — `crew_member_id` and `role` required:

```json
{ "crew_member_id": 12, "role": "first_mate", "patent_number": null }
```

### Trip enrollment (self-service)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/trips/{tripID}/enroll-token` | owner | Generate share token → `{token}` |
| DELETE | `/api/trips/{tripID}/enroll-token` | owner | Clear share token, 204 |
| GET | `/api/trips/{tripID}/enrollments` | owner | List enrollments |
| PUT | `/api/trips/{tripID}/enrollments/{id}/status` | owner | Set status, 204 |
| DELETE | `/api/trips/{tripID}/enrollments/{id}` | owner | Delete enrollment, 204 |
| GET | `/api/enroll/{token}` | any user | Resolve token → trip or cruise |
| POST | `/api/enroll/{token}` | any user | Enroll self, 201 |

`GET /api/enroll/{token}` returns `{"kind": "trip" \| "cruise", ...}` —
the token may resolve to either. `404 invalid enrollment link` if unknown.

Enroll body: `{ "note": "optional message" }`.
Enroll conflicts return `409` (`already enrolled`,
`enrollment closed: trip is not planned`).

Status values: `pending`, `accepted`, `rejected`, `waitlisted`.

### Voyages (owner-scoped)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/voyages` | List voyages | 200 |
| POST | `/api/voyages` | Create voyage | 201 |
| GET | `/api/voyages/{id}` | Get voyage | 200 |
| PUT | `/api/voyages/{id}` | Update voyage | 204 |
| DELETE | `/api/voyages/{id}` | Delete voyage | 204 |
| GET | `/api/voyages/{voyageID}/crew` | List voyage crew | 200 |
| POST | `/api/voyages/{voyageID}/crew` | Assign crew member | 201 |
| DELETE | `/api/voyages/{voyageID}/crew/{assignmentID}` | Remove assignment | 204 |

Voyage body — `name` required; also accepts `year` and the sailing-log fields
(`hours_total`, `hours_sail`, `hours_engine`, `hours_over_6bf`, `miles`,
`days`, `tidal_waters`) plus the trip fields above.

### Voyage opinions (crew documents)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/voyages/{voyageID}/opinions` | List generated opinions | 200 |
| POST | `/api/voyages/{voyageID}/opinions` | Generate opinion document | 201 |
| GET | `/api/voyages/{voyageID}/opinions/{id}/download` | Download file | 200 (file) |
| DELETE | `/api/voyages/{voyageID}/opinions/{id}` | Delete opinion | 204 |

Generate body — `crew_member_id` required, `format` must be `pdf` or `docx`:

```json
{ "crew_member_id": 12, "format": "pdf" }
```

`404 crew member not assigned to this voyage` if the member has no assignment.

### Yachts (owner-scoped)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/yachts` | List yachts | 200 |
| POST | `/api/yachts` | Create yacht | 201 |
| GET | `/api/yachts/{id}` | Get yacht | 200 |
| PUT | `/api/yachts/{id}` | Update yacht | 204 |
| DELETE | `/api/yachts/{id}` | Delete yacht | 204 |

Body — `name` required: `{ "name": "Bavaria 46", "registration_no": null, "yacht_type": "sailing yacht" }`.

### Crew members (owner-scoped)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/crew` | List crew members | 200 |
| POST | `/api/crew` | Create crew member | 201 |
| GET | `/api/crew/{id}` | Get crew member | 200 |
| PUT | `/api/crew/{id}` | Update crew member | 204 |
| DELETE | `/api/crew/{id}` | Delete crew member | 204 |

Body — `full_name` required: `{ "full_name": "Jan Nowak", "email": null, "patent_number": null }`.

### Trainings (owner-scoped)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/trainings` | List trainings | 200 |
| POST | `/api/trainings` | Create training | 201 |
| GET | `/api/trainings/{id}` | Get training | 200 |
| PUT | `/api/trainings/{id}` | Update training | 204 |
| DELETE | `/api/trainings/{id}` | Delete training | 204 |

Body — `name` required: `{ "name": "ISSA Skipper", "date": "2026-03-01", "organizer": null, "cost": 1200.0, "url": null }`.

### Uploads

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| POST | `/api/upload/image` | Multipart image upload (≤5 MB; jpeg/png/webp) → `{url}` | 200 |
| GET | `/api/uploads/*` | Serve a previously uploaded file | 200 (file) |

`400` on missing `file` field, oversized file, or unsupported type.

### Import (XLSX)

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| POST | `/api/import/xlsx` | Upload XLSX, returns parsed preview (`voyages`, `trainings`) | 200 |
| POST | `/api/import/confirm` | Persist the reviewed preview | 201 |

`confirm` body echoes the preview shape: `{ "voyages": [...], "trainings": [...] }`.
Returns `{voyages_created, trainings_created, yachts_created, crew_created}`.

### Organizations

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/orgs` | member | List the caller's orgs | 200 |
| POST | `/api/orgs` | any user | Create org (caller becomes admin) | 201 |
| GET | `/api/orgs/{slug}` | member | Get org | 200 |
| PUT | `/api/orgs/{slug}` | **admin** | Update org | 204 |
| DELETE | `/api/orgs/{slug}` | **admin** | Delete org | 204 |

Create/update body — `name` and (on create) `slug` required. `slug` is
lowercased and must match `^[a-z0-9]+(?:-[a-z0-9]+)*$`:

```json
{
  "name": "Warsaw Sailing Club", "slug": "warsaw-sailing",
  "description": null, "logo_url": null,
  "pzz_club_number": null, "city": "Warsaw", "website": null
}
```

`409 slug already taken` on a duplicate slug.

Org-resolution failures (`/api/orgs/{slug}/...`):

| Status | `error` value | Cause |
|--------|---------------|-------|
| 404 | `organization not found` | Unknown slug |
| 403 | `not a member of this organization` | Caller not a member |
| 403 | `insufficient permissions` | Member but not `admin` on an admin route |

### Org members & invites

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/orgs/{slug}/members` | member | List members |
| PUT | `/api/orgs/{slug}/members/{memberID}/role` | **admin** | Set role, 204 |
| DELETE | `/api/orgs/{slug}/members/{memberID}` | **admin** | Remove member, 204 |
| POST | `/api/orgs/{slug}/invites` | **admin** | Create invite, 201 |
| GET | `/api/orgs/{slug}/invites` | **admin** | List invites |
| DELETE | `/api/orgs/{slug}/invites/{inviteID}` | **admin** | Delete invite, 204 |
| GET | `/api/join/{token}` | any user | Invite info → `{org_name, org_slug, role, already_member}` |
| POST | `/api/join/{token}` | any user | Accept invite, 200 |

Roles: `admin`, `captain`, `crew`. The last `admin` cannot be demoted or
removed (`400`). Invite body:

```json
{ "role": "crew", "expires_in_hours": 168, "max_uses": 10 }
```

`expires_in_hours` and `max_uses` are optional; an unknown `role` defaults to
`crew`. Accepting an invite returns `410` when expired or fully used, and
`409 already a member` if the caller already belongs.

### Org resources (`/api/orgs/{slug}/...`)

All read routes require membership; create/update/delete require **admin**.

| Resource | Routes |
|----------|--------|
| Yachts | `GET \|POST /yachts`, `GET\|PUT\|DELETE /yachts/{id}` |
| Crew | `GET \|POST /crew`, `GET\|PUT\|DELETE /crew/{id}` |
| Trips | `GET \|POST /trips`, `GET\|PUT\|DELETE /trips/{id}`, `POST /trips/{id}/complete`, `POST /trips/{id}/cancel` |
| Trip crew | `GET\|POST /trips/{id}/crew`, `DELETE /trips/{id}/crew/{assignmentID}` |
| Voyages | `GET \|POST /voyages`, `GET\|PUT\|DELETE /voyages/{id}` |
| Voyage crew | `GET\|POST /voyages/{id}/crew`, `DELETE /voyages/{id}/crew/{assignmentID}` |
| Voyage opinions | `GET\|POST /voyages/{id}/opinions`, `GET /voyages/{id}/opinions/{opinionID}/download`, `DELETE /voyages/{id}/opinions/{opinionID}` |
| Cruises | `GET \|POST /cruises`, `GET\|PUT\|DELETE /cruises/{id}` |
| Dashboard | `GET /dashboard` |

Org trip/voyage crew assignment and voyage opinion routes mirror their
owner-scoped counterparts: reads require membership, mutations require
**admin**, and rows are scoped by `org_id`.

A **cruise** is an org-level event that groups child trips and voyages:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/orgs/{slug}/cruises/{id}/trips` | member | Child trips |
| GET | `/api/orgs/{slug}/cruises/{id}/voyages` | member | Child voyages |
| GET | `/api/orgs/{slug}/cruises/{id}/enrollments` | member | Cruise enrollments |
| POST | `/api/orgs/{slug}/cruises/{id}/enroll-token` | **admin** | Generate token → `{token}` |
| DELETE | `/api/orgs/{slug}/cruises/{id}/enroll-token` | **admin** | Clear token, 204 |
| PUT | `/api/orgs/{slug}/cruises/{id}/enrollments/{enrollmentID}/status` | **admin** | Set status, 204 |
| PUT | `/api/orgs/{slug}/cruises/{id}/enrollments/{enrollmentID}/trip` | **admin** | Assign enrollment to a trip, 204 |
| DELETE | `/api/orgs/{slug}/cruises/{id}/enrollments/{enrollmentID}` | **admin** | Delete enrollment, 204 |

Cruise enrollment status values: `pending`, `accepted`, `rejected`,
`waitlisted`. Assign-to-trip body: `{ "trip_id": 5 }` (`null` unassigns).
Cruise body — `name` required, plus `embark_date`, `disembark_date`,
`countries`, `start_port`, `end_port`, `description`, `image_*_url`,
`max_crew`, `cost_per_person`.
