# Sailor Buddy API

HTTP/JSON API for the Sailor Buddy backend. All application endpoints live
under `/api`. The frontend dev server proxies `/api` to the backend on
`:8080` (see `frontend/vite.config.ts`).

Sailor Buddy is a single-club app: all sailing data (cruises, trips, voyages,
yachts, crew) is shared across the club. Every member sees the same data;
only **admins** may create, edit or delete it.

## Authentication

Every `/api/*` endpoint requires a Firebase ID token:

```
Authorization: Bearer <firebase_id_token>
```

The `Auth` middleware verifies the token, then upserts a `users` row keyed by
Firebase UID (linking by verified email if the UID is new). The **first user
ever provisioned becomes the club admin**; every account after is a regular
`member`. Admins manage other members' roles on the members page.

Public endpoints (no token):

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness probe, returns `200` with empty body |
| `GET /api/enroll/{token}` | Enrollment preview — resolves a share token without auth |

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
| 200 | Success with body | `GET /api/trips` |
| 201 | Resource created | `POST /api/trips` |
| 204 | Success, no body | `PUT`/`DELETE` on most resources |
| 400 | Bad request — malformed body, failed validation, or unparseable path id | Missing `name`, `cannot demote the last admin` |
| 401 | Unauthorized — missing/invalid token | Missing `Authorization` header |
| 403 | Forbidden — authenticated but not an admin | Member calling a mutating route |
| 404 | Not found — resource missing | `GET /api/trips/999` |
| 409 | Conflict — uniqueness or state violation | Already enrolled |
| 500 | Internal server error | Database unavailable |

Notes:

- The whole API is served by the huma framework and described by
  `backend/openapi.yaml`. Malformed request bodies and unparseable path ids
  return `422` with a body-level error list.
- The error envelope is huma's RFC 9457 problem shape (`title`, `status`,
  `detail`, `errors`), not the legacy `{"error": "..."}` object.
- **Empty list responses serialize as `[]`**, never `null`.

## Pagination

Top-level collection list endpoints (`GET /api/trips`, `/voyages`, `/yachts`,
`/crew`, `/trainings`, `/cruises`) are offset-paginated. They accept two
query parameters:

| Param | Default | Bounds | Meaning |
|-------|---------|--------|---------|
| `limit` | `50` | `1`–`100` | Maximum items in the response window |
| `offset` | `0` | `>= 0` | Items skipped before the window |

Out-of-range values return `422`. The response is a page envelope, not a
bare array:

```json
{
  "items": [ ... ],
  "total": 327,
  "limit": 50,
  "offset": 100,
  "has_more": true
}
```

`total` is the full match count; `has_more` is `true` when more rows exist
beyond the window. `items` always serialises as `[]`, never `null`.

Nested sub-resource lists (a trip's crew, a cruise's child trips/voyages,
the members roster) are not paginated — they stay bare JSON arrays.

## Data model & authorization

All sailing data is **club-wide**: there is one club and every member sees
every cruise, trip, voyage, yacht and crew member. Authorization is by role,
held on the `users.role` column:

- **Reads** (`GET`) — open to any authenticated member.
- **Mutations** (`POST`/`PUT`/`DELETE`, plus `complete`/`cancel`/enroll-token)
  on shared club data — require the caller to be an `admin`; a member gets
  `403 admin only`.
- **Trainings** are the exception: a per-member course/certification log,
  scoped to the caller. Each member manages their own trainings.

Resources keep a `created_by` audit column (the user who created the row); it
is never used for access control. `crew_members` are decoupled from `users` —
crew need not have accounts.

Trip lifecycle: a `trip` is `planned`, then either `cancelled` or `completed`.
Completing a trip atomically converts it into a `voyage` (a logged, finished
sailing record) and repoints crew assignments.

## Endpoint reference

`{id}` path segments are integers; `{token}` is a string. Mutating routes
require the caller to be an `admin` (marked **admin** below).

### Identity

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Current user: `{id, email, name, avatar_url, role, patent_type, patent_number}` |
| PUT | `/api/auth/me` | Update the caller's sailing patent |
| GET | `/api/dashboard` | Club sailing stats + per-year breakdown |

### Members

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/members` | member | List club members `{id, name, email, role, ...}` |
| PUT | `/api/members/{userID}/role` | **admin** | Set a member's role, 204 |

Roles: `admin`, `member`. The last `admin` cannot be demoted (`400 cannot
demote the last admin`). Role body: `{ "role": "admin" }`.

### Trips

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/trips` | member | List trips | 200 |
| POST | `/api/trips` | **admin** | Create trip | 201 |
| GET | `/api/trips/{id}` | member | Get trip | 200 |
| PUT | `/api/trips/{id}` | **admin** | Update trip | 204 |
| DELETE | `/api/trips/{id}` | **admin** | Delete trip | 204 |
| POST | `/api/trips/{id}/complete` | **admin** | Complete trip → create voyage | 201 (voyage) |
| POST | `/api/trips/{id}/cancel` | **admin** | Cancel trip | 200 (trip) |

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

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/trips/{tripID}/crew` | member | List trip crew | 200 |
| POST | `/api/trips/{tripID}/crew` | **admin** | Assign crew member | 201 |
| DELETE | `/api/trips/{tripID}/crew/{assignmentID}` | **admin** | Remove assignment | 204 |

Assignment body — `crew_member_id` and `role` required:

```json
{ "crew_member_id": 12, "role": "first_mate", "patent_number": null }
```

### Trip enrollment (self-service)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/trips/{tripID}/enroll-token` | **admin** | Generate share token → `{token}` |
| DELETE | `/api/trips/{tripID}/enroll-token` | **admin** | Clear share token, 204 |
| GET | `/api/trips/{tripID}/enrollments` | **admin** | List enrollments |
| PUT | `/api/trips/{tripID}/enrollments/{id}/status` | **admin** | Set status, 204 |
| DELETE | `/api/trips/{tripID}/enrollments/{id}` | **admin** | Delete enrollment, 204 |
| GET | `/api/enroll/{token}` | public | Resolve token → trip or cruise |
| POST | `/api/enroll/{token}` | member | Enroll self, 201 |

`GET /api/enroll/{token}` returns `{"kind": "trip" \| "cruise", ...}` —
the token may resolve to either. `404 invalid enrollment link` if unknown.

Enroll body: `{ "note": "optional message" }`.
Enroll conflicts return `409` (`already enrolled`,
`enrollment closed: trip is not planned`).

Status values: `pending`, `accepted`, `rejected`, `waitlisted`.

### Voyages

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/voyages` | member | List voyages | 200 |
| POST | `/api/voyages` | **admin** | Create voyage | 201 |
| GET | `/api/voyages/{id}` | member | Get voyage | 200 |
| PUT | `/api/voyages/{id}` | **admin** | Update voyage | 204 |
| DELETE | `/api/voyages/{id}` | **admin** | Delete voyage | 204 |
| GET | `/api/voyages/{voyageID}/crew` | member | List voyage crew | 200 |
| POST | `/api/voyages/{voyageID}/crew` | **admin** | Assign crew member | 201 |
| DELETE | `/api/voyages/{voyageID}/crew/{assignmentID}` | **admin** | Remove assignment | 204 |

Voyage body — `name` required; also accepts `year` and the sailing-log fields
(`hours_total`, `hours_sail`, `hours_engine`, `hours_over_6bf`, `miles`,
`days`, `tidal_waters`) plus the trip fields above.

### Voyage ports

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/voyages/{voyageID}/ports` | member | List visited ports | 200 |
| POST | `/api/voyages/{voyageID}/ports` | **admin** | Add a port | 201 |
| DELETE | `/api/voyages/{voyageID}/ports/{portID}` | **admin** | Remove a port | 204 |
| PUT | `/api/voyages/{voyageID}/ports/order` | **admin** | Reorder ports | 200 |

### Voyage opinions (crew documents)

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/voyages/{voyageID}/opinions` | member | List generated opinions | 200 |
| POST | `/api/voyages/{voyageID}/opinions` | **admin** | Generate opinion document | 201 |
| GET | `/api/voyages/{voyageID}/opinions/{id}/download` | member | Download file | 200 (file) |
| DELETE | `/api/voyages/{voyageID}/opinions/{id}` | **admin** | Delete opinion | 204 |

Generate body — `crew_member_id` required, `format` must be `pdf` or `docx`:

```json
{ "crew_member_id": 12, "format": "pdf" }
```

`404 crew member not assigned to this voyage` if the member has no assignment.

### Cruises

A **cruise** is a club event that groups child trips and voyages (one per
yacht) and accepts cruise-level enrollments.

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/cruises` | member | List cruises | 200 |
| POST | `/api/cruises` | **admin** | Create cruise | 201 |
| GET | `/api/cruises/{id}` | member | Get cruise | 200 |
| PUT | `/api/cruises/{id}` | **admin** | Update cruise | 204 |
| DELETE | `/api/cruises/{id}` | **admin** | Delete cruise | 204 |
| GET | `/api/cruises/{id}/trips` | member | Child trips | 200 |
| GET | `/api/cruises/{id}/voyages` | member | Child voyages | 200 |
| POST | `/api/cruises/{id}/enroll-token` | **admin** | Generate token → `{token}` | 200 |
| DELETE | `/api/cruises/{id}/enroll-token` | **admin** | Clear token, 204 | 204 |
| GET | `/api/cruises/{id}/enrollments` | **admin** | List cruise enrollments | 200 |
| PUT | `/api/cruises/{id}/enrollments/{enrollmentID}/status` | **admin** | Set status, 204 | 204 |
| PUT | `/api/cruises/{id}/enrollments/{enrollmentID}/trip` | **admin** | Assign enrollment to a trip, 204 | 204 |
| DELETE | `/api/cruises/{id}/enrollments/{enrollmentID}` | **admin** | Delete enrollment, 204 | 204 |

Cruise body — `name` required, plus `embark_date`, `disembark_date`,
`countries`, `start_port`, `end_port`, `description`, `image_*_url`,
`max_crew`, `cost_per_person`. Cruise enrollment status values: `pending`,
`accepted`, `rejected`, `waitlisted`. Assign-to-trip body: `{ "trip_id": 5 }`
(`null` unassigns).

### Yachts

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/yachts` | member | List yachts | 200 |
| POST | `/api/yachts` | **admin** | Create yacht | 201 |
| GET | `/api/yachts/{id}` | member | Get yacht | 200 |
| PUT | `/api/yachts/{id}` | **admin** | Update yacht | 204 |
| DELETE | `/api/yachts/{id}` | **admin** | Delete yacht | 204 |

Body — `name` required: `{ "name": "Bavaria 46", "registration_no": null, "yacht_type": "sailing yacht" }`.

### Crew members

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/crew` | member | List crew members | 200 |
| POST | `/api/crew` | **admin** | Create crew member | 201 |
| GET | `/api/crew/{id}` | member | Get crew member | 200 |
| PUT | `/api/crew/{id}` | **admin** | Update crew member | 204 |
| DELETE | `/api/crew/{id}` | **admin** | Delete crew member | 204 |

Body — `full_name` required; also accepts `email`, `patent_number`, `phone`,
`pzz_license_type`, `pzz_license_number`, `emergency_contact_name`,
`emergency_contact_phone`.

### Trainings (per-member)

Each member keeps their own training/certification log.

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| GET | `/api/trainings` | List the caller's trainings | 200 |
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

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| POST | `/api/import/xlsx` | member | Upload XLSX, returns parsed preview (`voyages`, `trainings`) | 200 |
| POST | `/api/import/confirm` | **admin** | Persist the reviewed preview | 201 |

`confirm` body echoes the preview shape: `{ "voyages": [...], "trainings": [...] }`.
Returns `{voyages_created, trainings_created, yachts_created, crew_created}`.
