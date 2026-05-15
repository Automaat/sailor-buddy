# Sailor Buddy

Sailing cruise tracker and crew document generator.

## Stack

- **Backend**: Go 1.25 + chi router + PostgreSQL 18 (pgx/v5) + sqlc
- **Frontend**: SvelteKit 5 (Svelte 5 runes) + Tailwind CSS
- **Auth**: JWT access (15min) + refresh tokens (30d) + bcrypt

## Project Structure

```
backend/
  cmd/api/main.go          # entry point
  cmd/migrate/main.go       # standalone migration runner
  internal/
    api/router.go           # chi router setup
    api/handlers/            # HTTP handlers (auth, cruises, crew, yachts, trainings, dashboard, import)
    api/middleware/auth.go   # JWT middleware
    auth/jwt.go             # JWT + bcrypt helpers
    config/config.go         # env-based config
    db/db.go                # PostgreSQL connection + migration runner
    db/migrations/           # SQL migration files (001-008)
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
- Every API endpoint is huma-served and in the spec. Org routes resolve
  their scope via the `resolveOrg` helper (membership + role check),
  which replaced the former chi org middleware. Only static file
  serving (`GET /uploads/*`) stays on chi — it is not an API operation.

## Key Conventions

- sqlc-generated code in `db/sqlcdb/` is auto-generated - edit `db/queries/*.sql` instead
- All API routes under `/api/` - frontend proxies via vite config
- Auth routes (`/auth/*`) are public; all others require JWT
- Owner-scoped data: cruises, yachts, crew_members filtered by `owner_id`
- crew_members decoupled from users (crew may not have accounts)
- Go code must pass `gofumpt` formatting
- Env vars: SAILOR_DATABASE_URL, SAILOR_LISTEN_ADDR, SAILOR_UPLOAD_DIR, SAILOR_FIREBASE_PROJECT_ID
- `CORS_ALLOWED_ORIGINS`: comma-separated list of allowed CORS origins (required for production; default: `http://localhost:5173`)
