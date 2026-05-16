# Sailor Buddy

Sailing cruise tracker and crew document generator.

## Stack

- **Backend**: Go 1.25 + chi router + PostgreSQL 18 (pgx/v5) + sqlc
- **Frontend**: SvelteKit 5 (Svelte 5 runes) + Tailwind CSS
- **Auth**: Firebase ID tokens

## Quick start

```bash
docker compose up postgres   # local PostgreSQL
mise run dev-backend         # backend on :8080 (hot reload)
mise run dev-frontend        # frontend on :5173
```

## Documentation

- [`backend/API.md`](backend/API.md) — HTTP API reference: error envelope,
  status codes, authentication, and every endpoint.
- [`backend/openapi.yaml`](backend/openapi.yaml) — OpenAPI 3.1 spec derived
  from the Go DTOs; source for the generated frontend types.
- [`CLAUDE.md`](CLAUDE.md) — project structure, commands, and conventions.

## Tests

```bash
cd backend && go test ./...                       # backend
cd frontend && npm test && npx playwright test    # frontend unit + e2e
```
