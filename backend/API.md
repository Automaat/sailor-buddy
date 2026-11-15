# Sailor Buddy API

HTTP/JSON API for the Sailor Buddy backend. All application endpoints live
under `/api`. The frontend dev server proxies `/api` to the backend on
`:8080` (see `frontend/vite.config.ts`).

Sailor Buddy is a single-club app: all sailing data (cruises, trips, voyages,
yachts, crew) is shared across club accounts with a verified email. Emergency contacts and
opinions have narrower access. Mutations require **admin** access except for
the documented own-account, enrollment, training, and assigned-captain flows.

## Authentication

Every `/api/*` endpoint requires a Firebase ID token:

```
Authorization: Bearer <firebase_id_token>
```

The `Auth` middleware verifies the token, then reads the `users` row keyed by
Firebase UID. It upserts the row (linking by verified email if the UID is
new) only when the account is new or the token's email or name changed, so
a known account's request writes nothing. New accounts are members; club data
needs a verified email and an account that is not suspended, with no
administrator approval step. Bootstrap the first admin privately using
`bootstrap-admin`; HTTP sign-in never grants that role. See
[the access policy](../docs/security-access.md) and
[production session controls](../docs/production.md#disable-access-or-revoke-sessions).

Every token verification checks Firebase account disabling and revocation,
with a 5-second deadline and no application cache of account status. Provider
failures grant no authenticated access and return 503, so the client can retry
without losing its session.

Public endpoints (no token):

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness probe, returns `200` with empty body |
| `GET /readyz` | Readiness probe: pings Postgres (2 s timeout), returns `200`, or `503` when the database is unreachable |
| `GET /api/enroll/{token}` | Enrollment preview — resolves a share token without auth |

There is no `/auth/register` endpoint — account provisioning happens
automatically on the first authenticated request.

### Auth failures

| Status | Message (`error` or `detail`) | Cause |
|--------|---------------|-------|
| 401 | `missing authorization header` | No `Authorization` header |
| 401 | `invalid authorization format` | Header not `Bearer <token>` |
| 401 | `invalid or expired session` | Invalid/expired/revoked token, disabled or deleted Firebase account |
| 503 | `authentication service unavailable; try again` | Firebase/credential lookup failed; retry without signing out |
| 401 | `missing email claim` | Token has no email claim |
| 401 | `email not verified` | Email-link collision, email unverified |
| 409 | `email belongs to another account` | Verified email already used by another account that link-by-email cannot claim: it has another Firebase UID, or the token's UID already has an account of its own. No account is moved |
| 500 | `failed to provision user` | DB error during user upsert |

## Error responses

Every API response includes a server-generated `X-Request-ID` for correlation
with the private request/audit logs. CORS exposes it to allowed origins. See
[security logging](../docs/security-logging.md) for event fields and redaction.

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
body for its `401`, `409` and `500` responses via `http.Error`.

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
`/crew`, `/trainings`, `/cruises`, `/series`) are paginated. They accept these
query parameters:

| Param | Default | Bounds | Meaning |
|-------|---------|--------|---------|
| `limit` | `50` | `1`–`100` | Maximum items in the response window |
| `offset` | `0` | `>= 0` | Items skipped before the window (after the cursor, when `after` is set) |
| `after` | — | — | A previous page's `next_cursor`: the window starts after that page's last item |

Out-of-range values and a malformed `after` cursor return `422`. The response
is a page envelope, not a bare array:

```json
{
  "items": [ ... ],
  "total": 327,
  "limit": 50,
  "offset": 100,
  "has_more": true,
  "next_cursor": "eyJuIjoiQWxmYSIsImkiOjgxfQ"
}
```

`total` is the full match count; `has_more` is `true` when more rows exist
beyond the window, and `next_cursor` is then set (it is omitted on the last
page). `items` always serialises as `[]`, never `null`.

To read a whole list, pass each page's `next_cursor` as `after`. The cursor
carries the last item's sort key, so a row deleted before it shifts nothing;
with `offset` a delete moves the next row into the previous window and it is
never fetched. The cursor is opaque; do not build or parse it.

Nested sub-resource lists (a trip's crew, a cruise's child trips/voyages,
a series' editions, the members roster) are not paginated — they stay bare JSON arrays.

## Data model & authorization

All sailing data is **club-wide**: there is one club and every member sees
every cruise, trip, voyage, yacht and crew member. Authorization is by role,
held on the `users.role` column:

- **Reads** (`GET`) — open to any authenticated member, except hidden
  drafts (see Hidden drafts below).
- **Mutations** (`POST`/`PUT`/`DELETE`, plus `complete`/`cancel`/enroll-token)
  on shared club data — require the caller to be an `admin`; a member gets
  `403 admin only`.
- **Voyage captain** — the account linked to a voyage's captain person
  (`captain_person_id` → `crew_members.user_id`) may also write that
  voyage's post-cruise log (`PUT /voyages/{id}/log`), its visited ports, and
  add a yacht to the club list from the voyage
  (`POST /voyages/{id}/yachts`). A captain person without an account grants
  no rights. Other mutations stay admin-only.
- **Trainings** are the exception: a per-member course/certification log,
  scoped to the caller. Each member manages their own trainings.

Club attributes on `users`, both set by admins and not access roles:

- `is_captain` — membership of the club's captain group. The captain picker
  lists people whose account is in the group first, under their own
  heading. Joining requires `membership = czlonek` and a `patent_number` on
  the account (`422` otherwise).
- `membership` — `czlonek` (AGH student member), `sympatyk`
  (student of another university) or `gosc` (guest, the default for new accounts). A captain must stay
  `czlonek`.

Captain assignment: trip/voyage bodies take `captain_person_id`, a person
(`crew_members.id`). Any person may be chosen, with or without an account.
When set, the server overwrites `captain_name` with the person's name
(`422` for an unknown person). Without it, `captain_name` is kept as free
text (imported history). Deleting the person clears `captain_person_id` and
keeps `captain_name`. Completing a trip carries its captain to the voyage.

Resources keep a `created_by` audit column (the user who created the row); it
is never used for access control. `crew_members` are decoupled from `users` —
crew need not have accounts.

`crew_members` is the club's one list of people ("Osoby" in the UI).
Guarantee: every account has exactly one person, and a person links to at
most one account (`crew_members.user_id` is unique, `NULL` for people
without an account). The auth middleware creates the person on the
account's first authenticated request (`EnsureUserPerson`: name, or the
e-mail when the name is blank, plus e-mail, phone and patent number);
migration `024_people.sql` backfilled existing accounts the same way. An
account is never linked to an existing person automatically, not even by a
matching e-mail: linking waits for admin approval. `GetCrewMemberByUserID`
returns an account's person.

Every trip and voyage is a leg of a club event: `cruise_id` is required on
voyage bodies and on trip updates (`422 cruise not found` for an unknown id).
`POST /api/trips` without `cruise_id` creates a standalone etap: one
transaction creates a wrapper cruise without a series (with the trip's
name, countries and description) and the trip in it. A failed trip insert rolls the wrapper cruise back. A cruise
that still has trips or voyages cannot be deleted (`409`). A trip with cruise or
leg enrollments cannot be moved to another cruise (`409`).
A trip or voyage create or update, and a voyage log, with a `yacht_id` that
does not exist (or that is deleted during the save) returns
`422 yacht not found`.
Assigning crew or a cruise enrollment to a trip that was completed or
deleted at the same moment returns `409`.

Trip lifecycle: a `trip` is `planned`, then either `cancelled` or `completed`.
Completing a trip atomically converts it into a `voyage` (a logged, finished
sailing record) and repoints crew assignments.

#### Hidden drafts

Cruises, trips and voyages carry `published_at`, absent while the record is a
hidden draft. Every new cruise, trip and voyage starts hidden; records that
existed before migration `032_publication.sql` were published by it.

- A cruise is visible when it is published. A trip or voyage is visible when
  it **and its cruise** are published.
- Admins and the captain group (`users.is_captain`) see everything. Everyone
  else gets `404` for a hidden record and its nested routes (crew, ports,
  opinions, sign-ups), and hidden records are missing from every list,
  `total`, series-edition list and sea-time sum.
- Share links refuse hidden records for every caller: `GET`/`POST
  /enroll/{token}` answer `404` for a hidden cruise or trip, a cruise link
  lists only visible legs, and picking a hidden leg through it is `422`.
- `POST /trips/{id}/sign-up` on a hidden leg is `404`, or `422` for a caller
  who can see it.
- Completing a trip gives the voyage the trip's `published_at`.

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| POST | `/api/cruises/{id}/publish` | **admin** | Publish; optional body `{"include_legs": true}` also publishes its hidden trips and voyages in one transaction | 200 (cruise) |
| POST | `/api/cruises/{id}/unpublish` | **admin** | Hide the cruise, and with it all its legs | 200 (cruise) |
| POST | `/api/trips/{id}/publish` | **admin** | Publish; a hidden cruise without a series is published with it | 200 (trip) |
| POST | `/api/trips/{id}/unpublish` | **admin** | Hide the trip | 200 (trip) |
| POST | `/api/voyages/{id}/publish` | **admin** | Publish; a hidden cruise without a series is published with it | 200 (voyage) |
| POST | `/api/voyages/{id}/unpublish` | **admin** | Hide the voyage | 200 (voyage) |

Publishing keeps an earlier `published_at`. Unpublishing is allowed while the
record has enrollments. The captain group gets `403` like any non-admin.
A trip or voyage publish that races a move of the leg to another cruise
starts over on the new cruise; `409` means the leg kept moving on every
attempt.

## Endpoint reference

`{id}` path segments are integers; `{token}` is a string. Mutating routes
require the caller to be an `admin` (marked **admin** below).

### Identity

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Current user: `{id, email, name, first_name, last_name, phone, onboarded, avatar_url, role, is_captain, membership, patent_type, patent_number, person_id}`; `person_id` is the account's person |
| PUT | `/api/auth/me` | Save the caller's profile and finish onboarding (see below) |
| GET | `/api/auth/me/cruises` | `{cruise_ids}` of cruises the caller took part in: accepted cruise or trip enrollment, leg captained by the account's person, or crew-list entry linked to the account (or, for a person without an account, carrying its email) |
| GET | `/api/dashboard` | Caller's sea time; club totals for admins |

`PUT /api/auth/me` body — `first_name` and `last_name` required (non-blank);
optional `phone` (`+48 600 100 200` style), `patent_type` and
`patent_number`. A `patent_type` requires a `patent_number` (`422`); a
number without a type is dropped. A captain can't clear their patent number
(`422`). `name` becomes `first_name last_name`, and the first save sets
`onboarded`. After onboarding, sign-in no longer overwrites `name` with the
Firebase display name. The same statement copies the name, `phone` and
`patent_number` to the caller's person.

`GET /api/dashboard` returns `personal`: the caller's voyages as the
captain through their person (`captain_person_id`) or on a crew list through a crew
record linked to their account (`crew_members.user_id`). It has `total`,
`as_captain` and `as_crew` sums (`voyage_count`, `total_hours`,
`total_hours_sail`, `total_hours_engine`, `total_miles`, `total_days`),
`by_year` and a `voyages` list with `role` (`captain`|`crew`). A voyage
counts once; captain wins when both apply. Crew records without a linked
account don't count; an account's own person (created at sign-in) does.
Admins also get `club` with the club-wide `total` and
`by_year`; the key is absent for members.

### Members

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/members` | member | List club members `{id, name, email, role, is_captain, membership, onboarded, ...}` |
| PUT | `/api/members/{userID}/role` | **admin** | Set a member's role, 204 |
| PUT | `/api/members/{userID}/captain` | **admin** | Add to / remove from the captain group `{ "is_captain": true }`, 204 |
| PUT | `/api/members/{userID}/membership` | **admin** | Set membership status `{ "membership": "sympatyk" }`, 204 |
| PUT | `/api/members/{userID}/access` | **admin** | Suspend or restore an account `{ "suspended": true }`, 204 |

Roles: `admin`, `member`. The last `admin` cannot be demoted (`400 cannot
demote the last admin`). Role body: `{ "role": "admin" }`.

### Trips

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/trips` | member | List trips | 200 |
| POST | `/api/trips` | **admin** | Create trip; without `cruise_id` it gets its own wrapper cruise | 201 |
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
  "sign_up_deadline": "2026-05-15",
  "cruise_id": null
}
```

`sign_up_deadline` is the last day (`YYYY-MM-DD`, Europe/Warsaw, inclusive)
the leg takes sign-ups; omit it for no deadline. A deadline after
`embark_date` returns `422`, as does a value that is not a date. `PUT`
replaces the whole trip, so omitting it clears the deadline.

`complete` body (all optional; `year` falls back to the embark date):

```json
{
  "year": 2026, "hours_total": 56, "hours_sail": 40, "hours_engine": 16,
  "hours_over_6bf": 4, "miles": 320, "days": 7, "tidal_waters": 1
}
```

Trip prices are shown only; payments are not computed (admins record a
sign-up's payment status by hand, see Trip enrollment). All amounts are per
person, optional and `>= 0`, in `currency` (`PLN` or `EUR`, `PLN` when
omitted):

- `cost_per_person`: member price, for `czlonek` and `sympatyk`.
- `cost_per_person_non_member`: non-member price, for `gosc`. When only one
  of the two prices is set, everyone pays it.
- `yacht_fund`: estimated yacht fund (kasa jachtowa) for food, fuel and
  marinas.
- `deposit`: refundable deposit (kaucja).

An unknown currency or a negative amount is rejected with 422. `PUT`
replaces every price field, so omitting one clears it. What the price
covers (`included` / `not_included`) is set once on the trip's cruise.
Trips priced before migration 018 keep `cost_per_person` as the member
price in PLN. Completing a trip copies only `cost_total` and
`cost_per_person` to the voyage.

On create, an omitted (or 0) `max_crew` defaults to the yacht's `berths` when
`yacht_id` is set and the yacht records berths (`422 yacht not found` for an
unknown yacht). An explicit `max_crew` is kept.

`complete` returns `404` `trip not found or not in planned state` when the trip
is missing or already completed/cancelled, and `409`
`trip moved to another cruise; retry`, `trip captain changed; retry` or
`trip yacht changed; retry` when the trip keeps changing cruise, captain or
yacht while it completes. `cancel`
returns `404`
`trip not found or invalid transition` likewise.

### Trip crew assignments

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/trips/{tripID}/crew` | member | List trip crew | 200 |
| POST | `/api/trips/{tripID}/crew` | **admin** | Assign crew member; `404` when the person is gone, `409` when already on the crew | 201 |
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
| PUT | `/api/trips/{tripID}/enrollments/{id}/status` | **admin** | Set status, 204; `409` when accepting on a full leg (see Leg capacity) |
| PUT | `/api/trips/{tripID}/enrollments/{id}/payment-status` | **admin** | Set payment status → `Enrollment`, 200 |
| DELETE | `/api/trips/{tripID}/enrollments/{id}` | **admin** | Delete enrollment, 204 |
| GET | `/api/enroll/{token}` | public | Resolve token → trip or cruise |
| POST | `/api/enroll/{token}` | member | Enroll self via a trip token, 201; a cruise token returns `422` |
| POST | `/api/enroll/{token}/legs` | member | Enroll self in picked legs of the token's cruise, 201 |

`GET /api/enroll/{token}` returns `{"kind": "trip" \| "cruise", ...}` —
the token may resolve to either. `404 invalid enrollment link` if unknown.
Both kinds carry `trip_accepted`: `[{trip_id, accepted_count}]` for the
token's trip or each child trip of the cruise. It counts distinct users with
an accepted trip enrollment or an accepted cruise enrollment assigned to the
trip; the enroll page shows free places as `max_crew` minus that count (a
`max_crew` of 0 or null means no limit) and hides them when the count is
missing.
For a cruise, `cruise` carries the edition-wide fields: `countries`,
`description`, `image_photo_url`, `included`, `not_included`,
`travel_notes` and `min_age`. Dates, ports, crew limits and prices are on
its `trips`.
A cruise's `trips` are public leg summaries (`EnrollLeg`): name, status,
dates, ports, captain, `max_crew` and prices. They omit each trip's own
`enroll_token`, `created_by` and `cost_total`, so the cruise link never
reveals the per-leg links.

Enroll body: `{ "note": "optional message" }`.
Enroll conflicts return `409` (`already enrolled`,
`enrollment closed: trip is not planned`). Sign-ups are per leg: a cruise
token no longer takes a whole-cruise sign-up, so `POST /api/enroll/{token}`
with a cruise token returns `422` and writes nothing; legs are picked through
`POST /api/enroll/{token}/legs`. Existing `cruise_enrollments` rows stay and
are still managed through the cruise enrollment routes.

Both token previews carry each trip's `sign_up_deadline`. After it, both
token sign-ups return `422` and write nothing: `POST /api/enroll/{token}`
for that trip, and `POST /api/enroll/{token}/legs` when any picked leg is
past its deadline.

Both token sign-ups re-check the presented token under the lock they write
under, so a link an admin switches off or regenerates while a sign-up is in
flight takes no sign-up and writes nothing. `POST /api/enroll/{token}`
locks the trip and returns `409 enrollment closed: the enrollment link was
switched off` when the trip no longer has that token.
`POST /api/enroll/{token}/legs` share-locks the cruise while it still has
that token, then its legs, and returns `404 invalid enrollment link` when it
no longer does. A clear or regenerate that comes second waits for the
sign-up to commit.

Payment status: every trip enrollment carries `payment_status`, set by hand
by an admin after checking transfers: `unpaid` (default for a new sign-up),
`deposit` (first payment received) or `paid` (paid in full). No amounts are
stored. Body: `{ "payment_status": "deposit" }`. It is independent of
`status`: changing one leaves the other unchanged. `404 enrollment not
found` when the trip has no enrollment with that id. Cruise-level
enrollments omit `payment_status`.

### Leg sign-up (one click)

A signed-in user signs up for a leg (trip) of a cruise without a share
token. A sign-up is a `trip_enrollments` row. Any signed-in account may sign
up, including `gosc` and users who have not finished onboarding; requests
without a valid token get `401` from the auth middleware.

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/trips/{tripID}/sign-up` | member | Caller's sign-up state for the leg → `LegSignUp` | 200 |
| POST | `/api/trips/{tripID}/sign-up` | member | Sign up, no body → `{trip_id, result, enrollment}` | 201 / 200 |
| PUT | `/api/trips/{tripID}/sign-up/note` | member | Set the note on the caller's sign-up → `Enrollment` | 200 |
| DELETE | `/api/trips/{tripID}/sign-up` | member | Withdraw the caller's pending or waitlisted sign-up | 204 |
| GET | `/api/cruises/{cruiseID}/leg-sign-ups` | member | `LegSignUp` for every trip of the cruise | 200 |
| GET | `/api/cruises/{cruiseID}/trip-enrollments` | **admin** | Every leg sign-up of the cruise with the user, by leg then sign-up time | 200 |

`LegSignUp` is `{trip_id, open, accepted_count, enrollment?,
waitlist_position?}`: `open` is true while the leg is `planned`, its
sign-ups are switched on (the trip has an enroll token, set with
`POST /api/trips/{tripID}/enroll-token`) and its `sign_up_deadline`, if
any, is today or later in Europe/Warsaw;
`accepted_count` counts the same way as `trip_accepted` above; `enrollment`
is the caller's own sign-up and is omitted when there is none;
`waitlist_position` is set only while that sign-up is `waitlisted`.

`POST /api/trips/{tripID}/sign-up` creates an enrollment with no note
(`201`, `result: "created"`). It starts `pending`, or `waitlisted` with its
`waitlist_position` when the leg is full (see Leg capacity). Signing up
again returns the existing enrollment (`200`, `result: "already_enrolled"`), whatever its status and
even after the leg closed. A new sign-up on a leg that is not open returns
`422` and writes nothing. An unknown trip returns `404`; completing a leg
turns it into a voyage, so completed legs also return `404`.

The note is added after signing up: `PUT .../sign-up/note` with
`{ "note": "message" }` (up to 2000 characters, trimmed; blank or omitted
clears it). `404 not signed up for this leg` without a sign-up.

`DELETE .../sign-up` withdraws a `pending` or `waitlisted` sign-up with no
recorded payment. `404` when the caller has none, `409` once an admin has
accepted or rejected it, or set its payment status to `deposit` or `paid`.

A cruise enrollment made before per-leg sign-up and assigned to a leg counts
as the user's sign-up for that leg: both reads return it as `enrollment`
(it carries `cruise_id`), and `POST .../sign-up` returns it as
`already_enrolled` without adding a trip enrollment. Admins manage it through
the cruise enrollment routes, so the note and withdraw operations return
`409` for it.

Status values: `pending`, `accepted`, `rejected`, `waitlisted`.

### Leg capacity and waiting list

A leg's `max_crew` caps its accepted crew; `max_crew` unset or `0` means no
limit. Accepted crew counts distinct users with an accepted trip enrollment
or an accepted cruise enrollment assigned to the leg, as `accepted_count`.

- Accept: `PUT /api/trips/{tripID}/enrollments/{id}/status` with
  `accepted` returns `409 leg is full: all N places are taken` when other
  users' accepted crew already fills `max_crew`, and changes nothing. The
  trip row is locked for the change, so concurrent accepts for the last
  place serialize and the later one gets `409`. Other status changes,
  including from `accepted`, always apply. `404` when the enrollment does
  not exist or belongs to another trip.
- Waitlist threshold: a new leg sign-up (`POST .../sign-up`,
  `POST /api/enroll/{token}/legs`, `POST /api/enroll/{token}` with a trip
  token) starts `waitlisted` when accepted plus pending users already hold
  `max_crew` places, or when anyone is already waitlisted on the leg, so a
  newcomer never gets ahead of an earlier sign-up. Otherwise it starts
  `pending`. Pending sign-ups count because they are the queue an admin
  reviews first; rejected and waitlisted ones hold no place. All three
  paths lock the leg, so concurrent sign-ups count one another.
- Position: `waitlist_position` (1, 2, ...) orders a leg's waitlisted trip
  enrollments by sign-up time, then id. Payment does not change it.
- Promotion is manual: when a place frees up (an accepted sign-up is
  rejected or deleted), the admin accepts the first waitlisted person. No
  one is accepted automatically.

Cruise enrollments made before leg sign-ups follow the same limit: setting
one assigned to a leg to `accepted` (`PUT /api/cruises/{id}/enrollments/
{enrollmentID}/status`), or assigning an accepted one to another leg
(`PUT .../trip`), returns `409 leg is full` when the leg has no free place.
Both lock the leg like the trip enrollment accept; `404` when the
enrollment belongs to another cruise.

### Crew from accepted sign-ups

Accepting a sign-up for a leg, a trip enrollment or a cruise enrollment
assigned to the leg, puts the user's person on the leg's crew list
(`/api/trips/{tripID}/crew`) with role `zalogant`, in the same transaction
as the status change. A person already on the list keeps their entry. The
entry carries `crew_assignments.added_by_acceptance`, which is not exposed
in the API. Setting the sign-up to any other status, deleting it, or moving
an accepted cruise enrollment off the leg removes that entry once the user
holds no other accepted sign-up for the leg. Crew the admin added by hand
stays. A refused accept (`409`) changes nothing. Pending and waitlisted
sign-ups add no crew. Acceptance waits for a concurrent merge or delete of
the user's person; when the person still changed under it, or it kept
losing deadlocks to merges, it answers a retryable `409`.

Every status change, leg move and delete of a sign-up locks the legs it
touches before the enrollment row, so concurrent changes for one leg apply
one at a time. A cruise enrollment change that finds the enrollment moved
to another leg meanwhile rolls back and starts again; after three such
moves it returns `409`.

### Voyages

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/voyages` | member | List voyages | 200 |
| POST | `/api/voyages` | **admin** | Create voyage (body needs `cruise_id`) | 201 |
| GET | `/api/voyages/{id}` | member | Get voyage | 200 |
| PUT | `/api/voyages/{id}` | **admin** | Update voyage | 204 |
| PUT | `/api/voyages/{id}/log` | **admin** or voyage captain | Post-cruise log: hours, miles, days, tidal waters, `captain_notes`, `yacht_id`, `track_url`, `voyage_card`; every field replaces the stored value | 204 |
| POST | `/api/voyages/{id}/yachts` | **admin** or voyage captain | Add a yacht to the club list (does not assign it) | 201 |
| DELETE | `/api/voyages/{id}` | **admin** | Delete voyage | 204 |
| GET | `/api/voyages/{voyageID}/crew` | member | List voyage crew | 200 |
| POST | `/api/voyages/{voyageID}/crew` | **admin** | Assign crew member; `404` when the person is gone, `409` when already on the crew | 201 |
| DELETE | `/api/voyages/{voyageID}/crew/{assignmentID}` | **admin** | Remove assignment | 204 |

Voyage body — `name` required; also accepts `year` and the sailing-log fields
(`hours_total`, `hours_sail`, `hours_engine`, `hours_over_6bf`, `miles`,
`days`, `tidal_waters`) plus the trip fields above, `track_url` (absolute
`http`/`https` link to the track recording, else 422) and `voyage_card`
(free text: who holds the voyage card or where it is).

### Voyage ports

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/voyages/{voyageID}/ports` | member | List visited ports | 200 |
| POST | `/api/voyages/{voyageID}/ports` | **admin** or voyage captain | Add a port | 201 |
| DELETE | `/api/voyages/{voyageID}/ports/{portID}` | **admin** or voyage captain | Remove a port | 204 |
| PUT | `/api/voyages/{voyageID}/ports/order` | **admin** or voyage captain | Reorder ports | 200 |

Port body — `name` required; `latitude` and `longitude` are optional but come
together (one without the other is 422). A name-only port is listed in the
route but not drawn on the map. `anchorage: true` marks an anchorage.

### Port search

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/ports/suggest?q=` | member | Port names the club already entered | 200 |
| GET | `/api/geocode?q=` | member | Search towns in OpenStreetMap | 200 |

Both take `q` (required, at least 2 visible characters; surrounding blanks
and invisible characters are trimmed). A bare `q`, a blank one or a
one-character one is `422`.

**Suggestions** (`/ports/suggest`) search the `voyage_ports` table, ignoring
case and accents and tolerating small typos (PostgreSQL `pg_trgm` +
`unaccent`): `swinou` finds Świnoujście, `Jastarn` Jastarnia, `Svolvar`
Svolvær. Names starting with the query come first, then by similarity, then
by use count. Spellings that differ only in case or accents collapse into one
row named after the most used spelling. `limit` is 1–20 (default 8). Callers
who cannot see hidden voyages (everyone but admins and the captain group)
get names only from published voyages of published cruises.

```json
[{ "name": "Hel", "latitude": 54.6, "longitude": 18.8, "uses": 3 },
 { "name": "Zatoka Pucka", "uses": 1 }]
```

`latitude`/`longitude` come from the latest port of that name with a
location; a name only ever entered without one has none.

**Geocoding** (`/geocode`) proxies
[Nominatim](https://nominatim.org/release-docs/latest/api/Search/) and returns
up to 5 settlements:

- Kept: `place` records of type city, town, village, hamlet, suburb, island,
  islet, locality or municipality, and `boundary=administrative` records whose
  address type is one of those. Streets, POIs, marinas, natural features and
  regions are dropped.
- Names come in Polish where OpenStreetMap has them, else English, else the
  local name (`accept-language=pl,en`). `label` is "Town, County, Region,
  Country".
- Duplicates collapse to the best-ranked record: same name and full
  address (case- and accent-insensitive), or same name within 5 km.
- Optional `lat` and `lon` (map centre, both or neither, else `422`) rank
  nearby towns first without excluding others. The centre is rounded to a
  0.5° grid and sent as a Nominatim `viewbox` (±2° lat, ±3° lon) with
  `bounded=0`.
- When Nominatim finds no settlement or fails (anything but `429`), the proxy
  asks [Photon](https://photon.komoot.io) (`osm_tag=place`, English names,
  same bias), which matches prefixes and typos. `source` on each result is
  `nominatim` or `photon`.
- Answers are cached for 24 h by normalized query and rounded bias.
  Nominatim calls are limited to 1/s, Photon calls to 2/s. A request that
  would queue longer than 5 s, or an upstream `429`, answers
  `503 geocoding service busy, try again` without falling back. Other
  upstream failures answer `502`. A failed Photon fallback answers the same
  way (`503` when throttled, else `502`), never an empty list, and is not
  cached.

```json
[{ "name": "Jastarnia", "label": "Jastarnia, Puck County, Pomeranian Voivodeship, Poland",
   "latitude": 54.696, "longitude": 18.679, "source": "photon" }]
```

Results are © OpenStreetMap contributors (ODbL); the picker shows that
attribution, plus "Photon (komoot)" when a result came from Photon. Both
services are called with the User-Agent
`sailor-buddy/1.0 (+https://github.com/Automaat/sailor-buddy)` and are never
queried as the user types; only `/ports/suggest` is.

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

A **cruise** is a club event (an edition, edycja) that groups child trips
and voyages (etapy). It holds only what all its legs share; dates, ports,
captain, yacht, crew limit and prices belong to each leg, and the
frontend derives an edition's dates from its legs. Members sign up per leg
(see Leg sign-up); the cruise enroll token shares a public preview from
which legs are picked. Cruise-level enrollments made before that remain
listed and managed here.

The kind (`sea` or `inland`) belongs to the cruise's series (see Cruise
series); a cruise without a series counts as a sea cruise.

`GET /api/cruises` lists cruises newest first (by `id`).

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/cruises` | member | List cruises | 200 |
| POST | `/api/cruises` | **admin** | Create cruise edition (needs `series_id` and `edition`) | 201 |
| GET | `/api/cruises/{id}` | member | Get cruise | 200 |
| PUT | `/api/cruises/{id}` | **admin** | Update cruise | 204 |
| DELETE | `/api/cruises/{id}` | **admin** | Delete cruise; `409` while it has trips or voyages | 204 |
| GET | `/api/cruises/{id}/trips` | member | Child trips | 200 |
| GET | `/api/cruises/{id}/voyages` | member | Child voyages | 200 |
| POST | `/api/cruises/{id}/enroll-token` | **admin** | Generate token → `{token}` | 200 |
| DELETE | `/api/cruises/{id}/enroll-token` | **admin** | Clear token, 204 | 204 |
| GET | `/api/cruises/{id}/enrollments` | **admin** | List cruise enrollments | 200 |
| PUT | `/api/cruises/{id}/enrollments/{enrollmentID}/status` | **admin** | Set status; `409` when accepting on a full leg | 204 |
| PUT | `/api/cruises/{id}/enrollments/{enrollmentID}/trip` | **admin** | Assign enrollment to a trip; `409` when an accepted one moves to a full leg | 204 |
| DELETE | `/api/cruises/{id}/enrollments/{enrollmentID}` | **admin** | Delete enrollment, 204 | 204 |

Cruise body — `name` required, plus `countries`, `description`,
`image_*_url`, `included` / `not_included`, `series_id`, `edition` (see
Cruise series), `travel_notes` and `min_age`. `POST /api/cruises` creates
an edition of a series: it also requires `series_id` and a non-blank
`edition` (422 otherwise). `PUT` keeps both optional, so cruises without a
series stay editable.

- `included` / `not_included`: string lists of what every leg's price
  covers, at most 30 entries of up to 200 characters each. Entries are
  trimmed and blank ones dropped. Responses always carry both lists, `[]`
  when empty; more than 30 entries or a longer entry is rejected with 422.
- `travel_notes` — how to get there (flights, transport, meeting point),
  up to 5000 characters. Surrounding whitespace is trimmed and a blank
  value clears it.
- `min_age` — minimum crew age, 0–99. Information only: the app stores no
  dates of birth and does not check it.

Like the other optional fields, an omitted field resets to null on `PUT`.
Cruise enrollment status values: `pending`, `accepted`, `rejected`,
`waitlisted`. Assign-to-trip body: `{ "trip_id": 5 }`
(`null` unassigns).

### Cruise series

A **series** groups the yearly editions of a recurring cruise (for example
AGH Winter Sail Expedition or May I Sail). A cruise links to at most one
series with `series_id` and carries a free-form `edition` label up to 50
characters, such as `2025` or `2026/27`; a blank label is stored as null.
New cruises are created as editions (both fields required on `POST`);
cruises without a series are the wrapper cruises of standalone trips and
older events.
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
| GET | `/api/series/{id}/editions` | member | List the series' editions | 200 |

Body — `name` required, `kind` `sea` (rejs morski, the default) or
`inland` (rejs śródlądowy), shared by every edition of the series:
`{ "name": "May I Sail", "description": "Since 2018", "kind": "sea" }`.
Any other `kind` is rejected with 422; `PUT` resets an omitted `kind` to
`sea`.

`GET /api/series/{id}/editions` returns a bare array of the series' cruises,
newest first (by the earliest embark date among its trips and voyages the
caller may see, with editions that have no dated leg on top, then
`edition`). Each entry has `cruise_id`, `name`, `edition` and, once
published, `published_at`. Unknown series → 404.

```json
[
  {
    "cruise_id": 12,
    "name": "May I Sail 2025",
    "edition": "2025",
    "published_at": "2025-03-01T10:00:00Z"
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
| DELETE | `/api/yachts/{id}` | **admin** | Delete yacht; `409` while a trip or voyage uses it | 204 |

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

### Crew members (people)

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/crew` | member | List and search people | 200 |
| POST | `/api/crew` | **admin** | Create a person without an account | 201 |
| GET | `/api/crew/{id}` | member | Get a person | 200 |
| PUT | `/api/crew/{id}` | **admin** | Update a person | 204 |
| DELETE | `/api/crew/{id}` | **admin** | Delete a person without an account | 204 |
| GET | `/api/crew/{id}/merge-preview?duplicate_id=` | **admin** | Preview merging a duplicate into this person | 200 |
| POST | `/api/crew/{id}/merge` | **admin** | Merge a duplicate into this person | 200 |
| POST | `/api/crew/{id}/link` | **admin** | Link an account (`{ "user_id": 5 }`) to this person | 200 |
| POST | `/api/crew/{id}/unlink` | **admin** | Unlink this person from its account | 200 |

Body — `full_name` required; also accepts `other_names`, `email`,
`patent_number`, `phone`, `pzz_license_type`, `pzz_license_number`,
`emergency_contact_name`, `emergency_contact_phone`. `other_names` (other
spellings, maiden names; at most 20, each at most 200 characters, else
`422`) is trimmed, and blank or case-insensitively repeated entries are
dropped. PUT replaces the whole record, `other_names` included.

A person carries `has_account` (true when `user_id` is set) and
`other_names` (always an array). `GET /api/crew?q=` matches a
case-insensitive substring of `full_name`, any of `other_names`, or `email`
(the UI also folds Polish diacritics client-side); a blank `q` lists
everyone, and `total` counts the matches. Deleting a person linked to an
account returns `409`; it stays the account's person. Profile saves by the
account owner overwrite its person's name, phone and patent number.

#### Merging duplicates

`POST /api/crew/{id}/merge` with `{ "duplicate_id": 12 }` keeps person
`{id}` and removes the duplicate, in one transaction:

- Crew assignments (legs and voyages), voyage opinions and leg and voyage
  captaincies (`captain_person_id`) and "To ja" requests move to the kept person. Where both are on the same leg or voyage, the kept person's
  assignment stays (it takes the duplicate's patent number if it has none)
  and the duplicate's is dropped; the same applies to two opinions for one
  voyage, and the dropped opinion's file is deleted after commit.
- The duplicate's `full_name` and `other_names` join the kept person's
  `other_names` (repeats and the kept name skipped). Empty contact and
  licence fields are filled from the duplicate.
- The duplicate's account link moves to the kept person, together with
  the account person's `full_name` (profile saves overwrite it anyway); the
  kept person's own name then joins its `other_names`. Both linked to
  different accounts → `409`, nothing changes. `users` rows are never
  merged.
- The merge locks the moving account, both people, then every leg and
  voyage either person is on or captains (the person-then-leg order of leg
  completion and person deletion), so a concurrent profile save, leg
  completion or new crew assignment waits for it. Sign-up acceptance locks
  the leg, then the account's person; whichever side loses a deadlock
  between the two is retried up to three times. Go callers of
  `MergePeople` should retry on `40P01` too.
- An acceptance that waited for the merge adds the kept person, which now
  holds the account. Hand-assigning the duplicate, or any person deleted
  meanwhile, to a leg or voyage → `404`. Hand-assigning the kept person
  where the merge moves the duplicate's entry → `409`, as for any person
  already on the crew list; a deadlock with the merge is retried first.
- Where both are on the same leg, the kept entry counts as added by an
  accepted sign-up (`added_by_acceptance`) only when both entries did.
- The same ID twice, or merged `other_names` over the 20 × 200 limit →
  `422`; an unknown person → `404`.

Both endpoints return `{ person, duplicate, trip_assignments,
voyage_assignments, shared_assignments, opinions, shared_opinions,
captained_trips, captained_voyages }`: the
kept person after the merge, the duplicate, and the duplicate's row counts
(`shared_*` are the ones that collapse). The preview changes nothing and
fails the same way. Inside Go, `handlers.MergePeople` runs the merge on a
caller's transaction querier (`PlanPersonMerge` is the read-only variant).

#### Linking accounts and people ("To ja")

A user whose past voyages sit on a person without an account asks to be
linked to it; an admin approves. There is no instant self-link.

| Method | Path | Auth | Description | Success |
|--------|------|------|-------------|---------|
| GET | `/api/auth/me/person-candidates?q=` | member | Up to 20 people without an account; blank `q` lists only those with the account's e-mail | 200 |
| GET | `/api/auth/me/person-link-request` | member | `{ request }`: the caller's latest request, absent when none | 200 |
| POST | `/api/auth/me/person-link-request` | member | `{ "person_id": 12 }`: ask to be linked to that person | 201 |
| DELETE | `/api/auth/me/person-link-request` | member | Cancel the caller's pending request | 204 |
| GET | `/api/person-link-requests` | **admin** | Pending requests with user and person names and e-mails | 200 |
| GET | `/api/person-link-requests/pending-count` | **admin** | `{ count }` of pending requests | 200 |
| POST | `/api/person-link-requests/{id}/approve` | **admin** | Approve: link the account to the person | 200 |
| POST | `/api/person-link-requests/{id}/reject` | **admin** | Reject | 200 |

- Candidates match a case-insensitive part of `full_name` or
  `other_names`; e-mail matches (`email_match`) come first. Contact details
  are not returned.
- A request has `status` `pending`, `approved`, `rejected` or `cancelled`
  and `person_name`, the name the user claimed (the admin list shows the
  person's current name). Merging the requested person into another moves
  its requests to the kept person.
  An account has at most one pending request: another one → `409`. A
  person that already has an account → `409`; an unknown one → `404`.
  Cancelling without a pending request → `404`.
- Approving, and an admin's direct link, merge the account's own person
  into the requested one (`MergePeople`), so its voyages, opinions and
  captaincies follow and the requested person carries the account. A
  request no longer pending, or a person linked meanwhile → `409`. A
  direct link also closes the account's pending request: approved when it
  asked for that person, rejected otherwise. A person with another account
  → `409`.
- Unlinking clears the person's account link and its e-mail when it is the
  account's (otherwise it would still count toward `/api/auth/me/cruises`);
  the account gets a new person built from its profile, without history.
  An approved request for that person no longer shows in
  `/api/auth/me/person-link-request`, so the user can ask again. A person
  without an account → `409`.

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
| POST | `/api/upload/image` | Exactly one `file` part (≤5 MiB; jpeg/png/webp), no other fields; total body ≤5 MiB + 64 KiB framing. Oversized bodies return 413; extra parts return 422. → `{url}` | 200 |
| POST | `/api/upload/images/cleanup` | Clean my expired, never-attached managed images; no body | 204 |
| GET | `/api/uploads/*` | Serve a previously uploaded file | 200 (file) |

Images must fully decode and fit within 4096 pixels per side and four million
pixels total. Per-account storage defaults to 100 files/100 MiB, with a shared
20-request/minute upload/cleanup limit. Quota errors return 409; rate errors 429
with `Retry-After: 60`. See [upload storage](../docs/upload-storage.md) for finite
configuration overrides, reference protection and cleanup rules.

`400` on missing `file` field, oversized file, or unsupported type.

## Account admission

Club routes require a verified Firebase email and an account that is not suspended. There is no approval step: a new account with a verified email reads club data at once. Suspended accounts receive 403 on authenticated requests, including the self-service routes and requests with an existing token. Unverified users retain GET/PUT `/api/auth/me` and the existing token preview/enrollment routes. Group-chat links are omitted from their token previews. Admission errors include `code` and `detail`: `email_unverified` or `account_suspended`; suspension is checked first.

`GET /api/auth/me` includes `suspended` and the current token's `email_verified`. Member list entries include `suspended`. Admins suspend or restore an account with `PUT /api/members/{userID}/access` and JSON `{ "suspended": true }` (`false` restores it); successful updates return 204, an unknown account 404, and a non-admin caller 403. Suspending an admin account returns 409 until it is demoted, so the last admin can never be suspended. Promoting a suspended account also returns 409.

Crew list/detail responses omit emergency contacts unless the caller is an admin or captains a leg/voyage containing that person. Opinion lists are filtered to the caller's documents unless they are an admin or the voyage captain; unauthorized opinion downloads return 403. Normal shared club records remain available to every non-suspended user with a verified email. See `docs/security-access.md` for the full matrix.
