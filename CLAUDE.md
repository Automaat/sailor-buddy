# Sailor Buddy

Sailing cruise tracker and crew document generator.

## Stack

- **Backend**: Go + chi router + PostgreSQL 18 (pgx/v5) + sqlc
- **Frontend**: SvelteKit + Svelte 5 (runes) + Tailwind CSS
- **Auth**: Firebase Auth (ID token verified server-side); `users.role`
  (`admin`|`member`) — first registered user becomes admin
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
    api/middleware/auth.go   # Firebase auth middleware + first-user-admin
    auth/firebase.go         # Firebase client + Claims
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
cd backend && go run cmd/api/main.go  # run directly

# Frontend
cd frontend && npm run dev            # dev server on :5173
cd frontend && npm run build          # production build
cd frontend && npx svelte-check       # type check

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
  NOT NULL. The UI has one "Rejsy" list (`/cruises`) of events with
  Nadchodzące/Zrealizowane/Wszystkie tabs; trips and voyages are legs shown
  on the event page. There are no standalone trip/voyage list pages.
- Roles on `users.role` (`admin`|`member`); first registered user becomes
  admin. Admins are the board (Zarząd) and the Opiekun. Reads open to any
  member; mutations require admin (`requireAdmin`).
- Captains: `users.is_captain` (admin-curated captain group; requires
  `membership = czlonek` and a patent number). A leg's or voyage's captain is
  a person (`captain_person_id` → `crew_members`); any person may be chosen.
  The account linked to a voyage's captain person may write its post-cruise
  log and ports (`requireAdminOrCaptain`). Captain is not a role — keep it
  off `users.role`.
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
- `SAILOR_CHROME_NO_SANDBOX`: `true` turns off the Chromium sandbox for PDF
  renders. Only the root dev container sets it; see README "Production image".
- `CORS_ALLOWED_ORIGINS`: comma-separated list of allowed CORS origins (required for production; default: `http://localhost:5173`; a `*` is allowed only in the first host label under a registrable domain, such as `https://*.example.com` or `https://app-*.example.com`, or as the port of a named host, such as `http://localhost:*`; otherwise the API refuses to start)
