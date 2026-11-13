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
`/crew`, `/trainings`, `/cruises`, `/series`) are offset-paginated. They accept two
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
a series' editions, the members roster) are not paginated — they stay bare JSON arrays.

## Data model & authorization

All sailing data is **club-wide**: there is one club and every member sees
every cruise, trip, voyage, yacht and crew member. Authorization is by role,
held on the `users.role` column:

- **Reads** (`GET`) — open to any authenticated member.
- **Mutations** (`POST`/`PUT`/`DELETE`, plus `complete`/`cancel`/enroll-token)
  on shared club data — require the caller to be an `admin`; a member gets
  `403 admin only`.
- **Voyage captain** — the user assigned as a voyage's `captain_user_id` may
  also write that voyage's post-cruise log (`PUT /voyages/{id}/log`) and its
  visited ports. Other mutations stay admin-only.
- **Trainings** are the exception: a per-member course/certification log,
  scoped to the caller. Each member manages their own trainings.

Club attributes on `users`, both set by admins and not access roles:

- `is_captain` — membership of the club's captain group. Only users in the
  group can be assigned as a trip/voyage captain. Joining requires
  `membership = czlonek` and a `patent_number` on the account (`422`
  otherwise).
- `membership` — `czlonek` (AGH student member), `sympatyk`
  (student of another university) or `gosc` (guest, the default for new accounts). A captain must stay
  `czlonek`.

Captain assignment: trip/voyage bodies take `captain_user_id`. When set, the
server checks the captain is eligible (`422` otherwise) and overwrites
`captain_name` with the user's name. Without it, `captain_name` is kept as
free text (imported history).

Resources keep a `created_by` audit column (the user who created the row); it
is never used for access control. `crew_members` are decoupled from `users` —
crew need not have accounts.

Every trip and voyage is a leg of a club event: `cruise_id` is required on
trip and voyage bodies (`422 cruise not found` for an unknown id). A cruise
that still has trips or voyages cannot be deleted (`409`). A trip with cruise or
leg enrollments cannot be moved to another cruise (`409`).
Assigning crew or a cruise enrollment to a trip that was completed or
deleted at the same moment returns `409`.

Trip lifecycle: a `trip` is `planned`, then either `cancelled` or `completed`.
Completing a trip atomically converts it into a `voyage` (a logged, finished
sailing record) and repoints crew assignments.

## Endpoint reference

`{id}` path segments are integers; `{token}` is a string. Mutating routes
require the caller to be an `admin` (marked **admin** below).

### Identity

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Current user: `{id, email, name, first_name, last_name, phone, onboarded, avatar_url, role, is_captain, membership, patent_type, patent_number}` |
| PUT | `/api/auth/me` | Save the caller's profile and finish onboarding (see below) |
| GET | `/api/dashboard` | Club sailing stats + per-year breakdown |

`PUT /api/auth/me` body — `first_name` and `last_name` required (non-blank);
optional `phone` (`+48 600 100 200` style), `patent_type` and
`patent_number`. A `patent_type` requires a `patent_number` (`422`); a
number without a type is dropped. A captain can't clear their patent number
(`422`). `name` becomes `first_name last_name`, and the first save sets
`onboarded`. After onboarding, sign-in no longer overwrites `name` with the
Firebase display name.

### Members

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/members` | member | List club members `{id, name, email, role, is_captain, membership, ...}` |
| PUT | `/api/members/{userID}/role` | **admin** | Set a member's role, 204 |
| PUT | `/api/members/{userID}/captain` | **admin** | Add to / remove from the captain group `{ "is_captain": true }`, 204 |
| PUT | `/api/members/{userID}/membership` | **admin** | Set membership status `{ "membership": "sympatyk" }`, 204 |

Roles: `admin`, `member`. The last `admin` cannot be demoted (`400 cannot
demote the last admin`). Role body: `{ "role": "admin" }`.

### Trips

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/trips` | member | List trips | 200 |
| POST | `/api/trips` | **admin** | Create trip (body needs `cruise_id`) | 201 |
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
  "currency": "PLN",
  "cost_per_person_non_member": 650.0,
  "yacht_fund": 300.0,
  "deposit": 200.0,
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

Prices (see Cruise prices below) work as on a cruise, without the
`included` / `not_included` lists. Completing a trip copies only `cost_total`
and `cost_per_person` to the voyage.

On create, an omitted (or 0) `max_crew` defaults to the yacht's `berths` when
`yacht_id` is set and the yacht records berths (`422 yacht not found` for an
unknown yacht). An explicit `max_crew` is kept.

`complete` returns `404` `trip not found or not in planned state` when the trip
is missing or already completed/cancelled, and `409`
`trip moved to another cruise; retry` when the trip keeps changing cruise
while it completes. `cancel` returns `404`
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
Both kinds carry `trip_accepted`: `[{trip_id, accepted_count}]` for the
token's trip or each child trip of the cruise. It counts distinct users with
an accepted trip enrollment or an accepted cruise enrollment assigned to the
trip; the enroll page shows free places as `max_crew` minus that count (a
`max_crew` of 0 or null means no limit) and hides them when the count is
missing.
For a cruise, `cruise` also carries `travel_notes` and `min_age`, and
`group_chat_url` only when the caller is logged in: the preview is public,
the chat link is for members.
A cruise's `trips` are public leg summaries (`EnrollLeg`): name, status,
dates, ports, captain, `max_crew` and prices. They omit each trip's own
`enroll_token`, `created_by` and `cost_total`, so the cruise link never
reveals the per-leg links.

Enroll body: `{ "note": "optional message" }`.
Enroll conflicts return `409` (`already enrolled`,
`enrollment closed: trip is not planned`).

Status values: `pending`, `accepted`, `rejected`, `waitlisted`.

### Voyages

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/voyages` | member | List voyages | 200 |
| POST | `/api/voyages` | **admin** | Create voyage (body needs `cruise_id`) | 201 |
| GET | `/api/voyages/{id}` | member | Get voyage | 200 |
| PUT | `/api/voyages/{id}` | **admin** | Update voyage | 204 |
| PUT | `/api/voyages/{id}/log` | **admin** or voyage captain | Post-cruise log: hours, miles, days, tidal waters, `captain_notes` | 204 |
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
| POST | `/api/voyages/{voyageID}/ports` | **admin** or voyage captain | Add a port | 201 |
| DELETE | `/api/voyages/{voyageID}/ports/{portID}` | **admin** or voyage captain | Remove a port | 204 |
| PUT | `/api/voyages/{voyageID}/ports/order` | **admin** or voyage captain | Reorder ports | 200 |

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

Each cruise has a `kind`: `sea` (rejs morski) or `inland` (rejs
śródlądowy). Create and update default it to `sea` when omitted; any other
value is rejected with 422.

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/cruises` | member | List cruises | 200 |
| POST | `/api/cruises` | **admin** | Create cruise | 201 |
| GET | `/api/cruises/{id}` | member | Get cruise | 200 |
| PUT | `/api/cruises/{id}` | **admin** | Update cruise | 204 |
| DELETE | `/api/cruises/{id}` | **admin** | Delete cruise; `409` while it has trips or voyages | 204 |
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
`max_crew`, the price fields below, `series_id`, `edition` (see Cruise
series), and the logistics fields below.

Cruise prices are shown only; nothing tracks payments. All amounts are per
person, optional and `>= 0`, in `currency` (`PLN` or `EUR`, `PLN` when
omitted):

- `cost_per_person`: member price, for `czlonek` and `sympatyk`.
- `cost_per_person_non_member`: non-member price, for `gosc`. When only one
  of the two prices is set, everyone pays it.
- `yacht_fund`: estimated yacht fund (kasa jachtowa) for food, fuel and
  marinas.
- `deposit`: refundable deposit (kaucja).
- `included` / `not_included`: string lists of what the price covers, at most
  30 entries of up to 200 characters each. Entries are trimmed and blank ones
  dropped. Responses always carry both lists, `[]` when empty.

An unknown currency, a negative amount, more than 30 entries or a longer
entry is rejected with 422. `PUT` replaces every price field, so omitting
one clears it. `GET /api/enroll/{token}` returns the same price fields on
its `trip` and `cruise` summaries; the lists come with the cruise only.
Cruises and trips priced before migration 018 keep `cost_per_person` as the
member price in PLN.

Logistics, all optional and returned on every cruise read:

- `group_chat_url` — link to the crew group chat (Discord, WhatsApp, ...).
  Must be an absolute `http` or `https` URL, up to 2000 characters, whose
  host a browser accepts (a valid IPv4 address if it ends in a number,
  valid IDNA labels, no IPv6 zone ID); anything else is rejected with 422.
- `travel_notes` — how to get there (flights, transport, meeting point),
  up to 5000 characters.
- `min_age` — minimum crew age, 0–99. Information only: the app stores no
  dates of birth and does not check it.

Surrounding whitespace is trimmed and a blank `group_chat_url` or
`travel_notes` clears the field. Like the other optional fields, an omitted
logistics field resets to null on `PUT`.
Cruise enrollment status values: `pending`, `accepted`, `rejected`,
`waitlisted`. Assign-to-trip body: `{ "trip_id": 5 }`
(`null` unassigns).

### Cruise series

A **series** groups the yearly editions of a recurring cruise (for example
AGH Winter Sail Expedition or May I Sail). A cruise links to at most one
series with `series_id` and carries a free-form `edition` label up to 50
characters, such as `2025` or `2026/27`; a blank label is stored as null.
A `series_id` that does not exist is rejected with 422. `PUT
/api/cruises/{id}` replaces both fields, so omitting them unlinks the
cruise. Deleting a series keeps its cruises and clears their `series_id`.

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/series` | member | List series | 200 |
| POST | `/api/series` | **admin** | Create series | 201 |
| GET | `/api/series/{id}` | member | Get series | 200 |
| PUT | `/api/series/{id}` | **admin** | Update series | 204 |
| DELETE | `/api/series/{id}` | **admin** | Delete series; its cruises stay, unlinked | 204 |
| GET | `/api/series/{id}/editions` | member | Editions with voyage totals | 200 |

Body — `name` required: `{ "name": "May I Sail", "description": "Since 2018" }`.

`GET /api/series/{id}/editions` returns a bare array of the series' cruises,
newest first (by `embark_date`, with undated planned editions on top, then
`edition`), each with `voyage_count`,
`total_miles` and `total_hours` summed over its voyages (`hours_total`).
An edition without voyages reports zeros. Unknown series → 404.

```json
[
  {
    "cruise_id": 12,
    "name": "May I Sail 2025",
    "edition": "2025",
    "kind": "sea",
    "embark_date": "2025-05-01",
    "voyage_count": 2,
    "total_miles": 200,
    "total_hours": 40
  }
]
```

### Yachts

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/yachts` | member | List yachts | 200 |
| POST | `/api/yachts` | **admin** | Create yacht | 201 |
| GET | `/api/yachts/{id}` | member | Get yacht | 200 |
| PUT | `/api/yachts/{id}` | **admin** | Update yacht | 204 |
| DELETE | `/api/yachts/{id}` | **admin** | Delete yacht | 204 |

Body — `name` required; the specs are optional:

```json
{
  "name": "Ilmatar", "registration_no": null, "yacht_type": "Gib'Sea 43",
  "berths": 10, "length_m": 12.8, "build_year": 2002,
  "engine": "Volvo Penta D2-55", "owner_name": "Szkoła Żeglarska Kliper"
}
```

`berths` must be at least 1, `length_m` above 0 and `build_year` within
1800–2100, otherwise `422`. `owner_name` is the charter owner (armator).

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
