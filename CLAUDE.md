# Sailor Buddy

Sailing cruise tracker and crew document generator.

## Stack

- **Backend**: Go + chi router + PostgreSQL 18 (pgx/v5) + sqlc
- **Frontend**: SvelteKit + Svelte 5 (runes) + Tailwind CSS
- **Auth**: Firebase Auth (ID token verified server-side); `users.role`
  (`admin`|`member`) — initial admin provisioned privately
- **Tool versions**: Go, Node, linters and codegen tools are pinned in
  `mise.toml`. Docs point there instead of repeating those versions.

## Project Structure

```
backend/
  cmd/api/main.go          # entry point
  cmd/migrate/main.go       # standalone migration runner
  internal/
    api/router.go           # chi router setup
    api/handlers/            # HTTP handlers: one file per resource, plus shared helpers
    api/handlers/authz.go    # requireAdmin role gate
    api/middleware/auth.go   # Firebase auth middleware
    auth/firebase.go         # Firebase client + Claims
    cli/cli.go               # -h / argument parsing for migrate and seed-dev
    config/config.go         # env-based config
    db/db.go                # PostgreSQL connection + migration runner
    db/migrations/           # SQL migrations, numbered NNN_name.sql, applied in order
    db/queries/              # sqlc SQL query files
    db/sqlcdb/               # generated sqlc Go code (DO NOT EDIT)
frontend/
  src/lib/api/              # API client + types
  src/lib/stores/auth.ts    # Svelte 5 runes auth store
  src/routes/               # SvelteKit pages
```

## Commands

```bash
# Backend
cd backend && go build ./...          # build
cd backend && go test ./...           # test
cd backend && mise exec -- air        # dev server with hot reload
cd backend && SAILOR_DATABASE_URL=... go run ./cmd/api  # run directly

# Frontend
cd frontend && npm run dev            # dev server on :5173
cd frontend && npm run build          # production build
cd frontend && npx svelte-check       # type check
cd frontend && npm run lint           # prettier --check + eslint
cd frontend && npm run format         # prettier --write

# Migrations (api and seed-dev also migrate on start)
mise run migrate                      # local dev DB on localhost:5432
cd backend && SAILOR_DATABASE_URL=... go run ./cmd/migrate

# sqlc (after editing queries/*.sql)
cd backend && mise exec -- sqlc generate

# Full stack
mise run dev-backend   # backend hot reload
mise run dev-frontend  # frontend dev
```

## API Reference

See `backend/API.md` for the error response envelope, status code
conventions, authentication format, and the full endpoint reference.

### OpenAPI / type generation

The API is served by the huma framework, which derives an OpenAPI 3.1
spec from Go types. Pipeline:

```
Go DTO structs (internal/api/dto) → huma → backend/openapi.yaml
  → openapi-typescript → frontend/src/lib/api/schema.d.ts
```

- API wire models live in `internal/api/dto/`, kept separate from the
  sqlc row structs so the HTTP contract is decoupled from the schema.
- huma operations are registered per resource (e.g. `RegisterTripRoutes`),
  wired up in `internal/api/humaapi.go`.
- Regenerate both artifacts after changing a DTO or operation:
  `mise run gen-api`. `openapi.yaml` and `schema.d.ts` are committed.
- Every API endpoint is huma-served and in the spec. Mutating routes gate on
  the caller's role via the `requireAdmin` helper (`internal/api/handlers/
  authz.go`). Only static file serving (`GET /uploads/*`) stays on chi — it
  is not an API operation.

## Key Conventions

- sqlc-generated code in `db/sqlcdb/` is auto-generated - edit `db/queries/*.sql` instead
- All API routes under `/api/` - frontend proxies via vite config
- Single club: all sailing data (cruises, trips, voyages, yachts, crew) is
  club-wide and shared. No per-user/per-org scoping.
- Every trip and voyage belongs to a cruise (club event): `cruise_id` is
  NOT NULL. The "Rejsy" nav item opens the series list (`/series`); each
  series page lists its editions, and trips and voyages are legs shown on
  the edition page. At the end of Rejsy, "Bez serii" lists every leg of a
  cruise without a series, plus such a cruise with no leg
  (`GET /standalone-legs`, `src/lib/rejsy.ts`), so none is unreachable.
  `/cruises` redirects to `/series`. There are no other trip/voyage list
  pages; the caller's own upcoming legs are on the dashboard.
- Seria (`cruise_series`) → Edycja (`cruises` with `series_id`) → Etap
  (`trips`/`voyages`). Edycje are created only from the series page
  (`/cruises/new?series_id=`); `POST /cruises` requires `series_id` and
  `edition`. Rejsy's "+ Nowy rejs" creates an etap (`/trips/new`): a trip
  may pick a Seria → Edycja or stay standalone, and `POST /trips` without
  `cruise_id` wraps it in its own cruise without a series. Cruises without a
  series stay valid and show under "Bez serii".
- An edycja (`cruises`) holds only what all its legs share: name, countries,
  description, images, `included`/`not_included`, travel notes, min age.
  Dates, ports, captain, yacht, crew limit and prices live on each etap;
  a series' editions are sorted by their legs' dates and the UI does not
  show an edition's dates. `kind` (sea/inland) is on `cruise_series`; a
  cruise without a series is sea.
- Roles on `users.role` (`admin`|`member`); initial admin is provisioned privately with `bootstrap-admin`. Admins are the board (Zarząd) and the Opiekun. Club reads require a verified Firebase email
  (no approval step); `users.suspended` denies access, existing tokens included. Emergency contacts are
  restricted to admins and assigned captains; opinions to admins, subject,
  and voyage captain. See `docs/security-access.md`. Mutations require admin
  (`requireAdmin`).
- Captains: `users.is_captain` (admin-curated captain group; requires
  `membership = czlonek` and a patent number). A leg's or voyage's captain is
  a person (`captain_person_id` → `crew_members`); any person may be chosen.
  The account linked to a voyage's captain person may write its post-cruise
  log and ports and generate and delete its opinions (`requireAdminOrCaptain`).
  A leg's captain sees its crew read-only with contacts (phone, emergency
  contact) and may complete the leg only after its `disembark_date`
  (Europe/Warsaw). Captain is not a role — keep it off `users.role`.
- Yacht list (`/yachts`, `GET /yachts*`): admins and the captain group only.
  A voyage captain outside the group picks a yacht through
  `GET /voyages/{id}/yachts`; members see a voyage's yacht as the
  `yacht` summary on `GET /voyages/{id}`. Do not expose the full list to
  members again.
- Hidden drafts: `published_at` on cruises, trips and voyages; NULL is
  hidden and every new row starts hidden. A trip or voyage is visible only
  while it and its cruise are published. Admins and the captain group see
  hidden records (`canSeeHidden`, `internal/api/handlers/authz.go`). Reads
  pass `include_hidden` to the sqlc query, never filter in Go after paging;
  admin-only paths pass `true`. Share links and sign-ups refuse hidden
  records for everyone. Publish/unpublish are admin-only
  (`handlers/publication.go`).
- `users.membership`: `czlonek` | `sympatyk` | `gosc` (club statute).
  Self-registered accounts start as `gosc`; an admin promotes them.
- Onboarding: `users.onboarded_at` NULL sends a signed-in user to
  `/onboarding` (layout gate, `src/lib/onboarding.ts`) to fill first/last
  name, optional phone and patent, then back to the original page. The
  profile page edits the same fields (`ProfileFields.svelte`).
- Trainings are the exception — a per-member log scoped by `user_id`.
- crew_members is the people list ("Osoby"); people may lack accounts, but
  every account has exactly one person (`crew_members.user_id` unique),
  created on first sign-in. Never auto-link an account to an existing person.
- Page loads: never swallow a failed section request with `.catch(() => [])`.
  Wrap it in `settle(p, fallback, noteFailure(loadErrors, '<label>'))`
  (`src/lib/api/errors.ts`) and render `LoadErrorBanner`, so an empty list
  from a failed request is not shown as real data.
- Page data loads in `+page.ts` (`export const load: PageLoad`), not in
  `onMount`: SvelteKit reuses the component between two URLs of one route,
  so `onMount` would keep the previous record. The component reads
  `let { data } = $props()`. Load the page's record with
  `found(p, { message, title, backHref, backLabel })` (`src/lib/api/load.ts`),
  which throws `error(404)` for the root `+error.svelte` to render — no
  per-page `notFound` flags. Sections the user can edit in place use a
  writable `$derived(data.x)`. Admin-only lists stay in an `$effect` keyed on
  the record id, since `auth.isAdmin` is false until `/auth/me` answers.
  Forms filled from load data use `let form = $derived(perLoad(data, …))`
  (`src/lib/state.svelte.ts`): the form, its error and busy flag start over
  on the next record, and a save acts on its reply only while its held state
  is still the current one. Wrap a form with a `YachtSelect` in
  `{#key data}` so a yacht still being added is dropped with the record.
  Migration of the remaining `onMount` pages is tracked in #600.
- Go code must pass `gofumpt` formatting
- Handler tests use `mockQuerier` (`internal/api/handlers/mock_test.go`),
  which embeds a nil `sqlcdb.Querier`: a new query needs no mock change to
  compile, and an unstubbed call panics. Add a mock method and `fooFn`
  field only when a test needs to stub that query.
- Env vars: SAILOR_DATABASE_URL, SAILOR_LISTEN_ADDR, SAILOR_UPLOAD_DIR, SAILOR_FIREBASE_PROJECT_ID
- `SAILOR_DATABASE_URL` has no default: `cmd/api`, `cmd/migrate` and
  `cmd/seed-dev` all migrate on start, so each exits before connecting when it
  is unset. `migrate` and `seed-dev` take only `-h`; any other argument exits 2.
- `SAILOR_CHROME_NO_SANDBOX`: `true` turns off the Chromium sandbox for PDF
  renders. Only the root dev container sets it; see README "Production image".
- `CORS_ALLOWED_ORIGINS`: comma-separated list of allowed CORS origins (required for production; default: `http://localhost:5173`; a `*` is allowed only in the first host label under a registrable domain, such as `https://*.example.com` or `https://app-*.example.com`, or as the port of a named host, such as `http://localhost:*`; otherwise the API refuses to start)
