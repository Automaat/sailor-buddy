# Sailor Buddy

Sailing cruise tracker and crew document generator.

## Stack

- **Backend**: Go 1.23 + chi router + PostgreSQL 18 (pgx/v5) + sqlc
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

## Key Conventions

- sqlc-generated code in `db/sqlcdb/` is auto-generated - edit `db/queries/*.sql` instead
- All API routes under `/api/` - frontend proxies via vite config
- Auth routes (`/auth/*`) are public; all others require JWT
- Owner-scoped data: cruises, yachts, crew_members filtered by `owner_id`
- crew_members decoupled from users (crew may not have accounts)
- Go code must pass `gofumpt` formatting
- Env vars: SAILOR_DATABASE_URL, SAILOR_LISTEN_ADDR, SAILOR_UPLOAD_DIR, SAILOR_FIREBASE_PROJECT_ID
