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

Use the [production configuration guide](docs/production.md) and
[`compose.production.yml`](compose.production.yml) for the runtime settings,
Firebase identity, TLS routing, and suspension procedure. Final images default
to production mode and reject emulator authentication.

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
- **AppArmor**: keep Docker's default AppArmor profile (`docker-default`).
  Hosts that restrict unprivileged user namespaces
  (`kernel.apparmor_restrict_unprivileged_userns=1`, the default from Ubuntu
  24.04) need no change with it. Do not add
  `--security-opt apparmor=unconfined`: on those hosts the kernel then moves
  Chromium into the `unprivileged_userns` profile, which denies the
  `CAP_SYS_ADMIN` the sandbox needs, and PDF renders fail with `Failed to move
  to new namespace`. The startup log names AppArmor when this happens. The
  seccomp profile is still required with either AppArmor setting.
  Verified on Ubuntu 24.04.5 (kernel 6.8, Docker 29.9).
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

## Initial administrator

Normal sign-in creates a member account and never promotes it. Sign in once with the trusted account, copy its exact UID from the Firebase console, and run `bootstrap-admin` privately with `SAILOR_DATABASE_URL` and `SAILOR_BOOTSTRAP_FIREBASE_UID` set through your secret manager. The production image includes `/app/bootstrap-admin`; local development can use `cd backend && mise exec -- go run ./cmd/bootstrap-admin`.

Run database migrations before bootstrap. The command requires an existing account with that exact UID, never links by email, and refuses to add another administrator if one exists. Repeating it for an already-admin account succeeds without changing accounts. Existing admins and normal role management are preserved. `-h` prints help without connecting. Never expose this command as an HTTP endpoint.

## Account approval

Registration creates a pending account. A user must verify their Firebase email and receive administrator approval before reading club data. The pending-access screen offers email verification and a refresh action; users can still edit their own profile and enroll through a shared link.

Administrators manage approval and suspension in the members table. Club membership and captain-group status do not approve an account. An administrator must be demoted before their account can be restricted, and a suspended member cannot be promoted to admin. Suspension blocks the next authenticated request, including requests using an existing ID token. Firebase token revocation checks are tracked separately.

The initial migration approves existing administrators only. Other accounts remain pending, as no real users existed at this policy decision. Emergency contacts and voyage opinions follow the access matrix in [the account access policy](docs/security-access.md).
