# Sailor Buddy

Sailing cruise tracker and crew document generator.

## Stack

- **Backend**: Go + chi router + PostgreSQL 18 (pgx/v5) + sqlc
- **Frontend**: SvelteKit + Svelte 5 (runes) + Tailwind CSS
- **Auth**: Firebase ID tokens

Tool versions (Go, Node, linters, codegen) are pinned in
[`mise.toml`](mise.toml); run `mise install` to get them.

## Quick start

```bash
docker compose up postgres   # local PostgreSQL
mise run dev-backend         # backend on :8080 (hot reload)
mise run dev-frontend        # frontend on :5173
```

## Production image

`backend/Dockerfile` builds the API image. It runs as the unprivileged user
`sailor` (UID and GID 10001) so Chromium renders PDF opinions with its
sandbox on.

- **Seccomp**: Docker's default seccomp profile blocks the namespaces the
  Chromium sandbox needs, and PDF renders then fail with `chrome failed to
  start`. The API logs `PDF renderer unavailable` at startup when that
  happens. Run the container with
  [`backend/seccomp-chromium.json`](backend/seccomp-chromium.json):
  `--security-opt seccomp=backend/seccomp-chromium.json`, or
  `security_opt: ["seccomp=backend/seccomp-chromium.json"]` in Compose. It is
  Docker's default profile ([moby/profiles](https://github.com/moby/profiles)
  `seccomp/v0.3.0`) plus one rule that allows `clone` and `unshare`.
- **Uploads**: `SAILOR_UPLOAD_DIR` (default `/app/uploads`) must be
  writable by UID 10001. A new named volume at `/app/uploads` gets that owner
  from the image. A bind mount, or a volume written by an older root image,
  needs `chown -R 10001:10001` on its files first. The API refuses to start
  while any directory or file there is not writable.
- `SAILOR_CHROME_NO_SANDBOX=true` turns the sandbox off. Only the dev image
  in `docker-compose.yml` sets it, because it runs as root.

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

## Lint and format

```bash
cd frontend && npm run lint     # prettier --check + eslint
cd frontend && npm run format   # prettier --write
```

ESLint keeps `svelte/no-navigation-without-resolve` off: the app is served at
the site root with no `paths.base`, so `resolve()` would not change any URL.
