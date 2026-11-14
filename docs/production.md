# Production configuration

The app has no production deployment yet. This example defines the runtime
contract; deployment verification stays open in #690 until the hosting target
and hostname exist.

## Runtime

Use [`compose.production.yml`](../compose.production.yml) on a host with a TLS
reverse proxy. Run `docker compose -f compose.production.yml config` to validate
the settings before `up -d`. Supply the variables through the hosting secret
manager or a protected operator environment. Do not commit credential files.

| Variable | Value |
| --- | --- |
| `SAILOR_BACKEND_IMAGE` | Final backend image pinned as `registry/image@sha256:…` |
| `SAILOR_FRONTEND_IMAGE` | Final frontend image pinned as `registry/image@sha256:…` |
| `SAILOR_PUBLIC_ORIGIN` | Public origin, e.g. `https://sailing.example.com`, without a trailing slash |
| `SAILOR_DATABASE_URL` | Private PostgreSQL connection string with TLS certificate verification (`sslmode=verify-full`) |
| `SAILOR_FIREBASE_PROJECT_ID` | Production Firebase project ID |
| `SAILOR_FIREBASE_API_KEY` | Firebase web app API key (public client configuration) |
| `SAILOR_FIREBASE_AUTH_DOMAIN` | Firebase web app auth domain, e.g. `club.firebaseapp.com` |
| `SAILOR_FIREBASE_ADC_FILE` | Readable workload identity credential configuration file on the host |

- The proxy routes `/api` and `/api/*` to `127.0.0.1:8080`, and the rest to
  `127.0.0.1:3000`. It terminates HTTPS, redirects HTTP to HTTPS, sets HSTS on
  HTTPS responses, and bounds request size and rate. Verify those settings on
  the chosen host in #690. Both app ports bind only to loopback.
- PostgreSQL runs on private networking; this example exposes no database,
  emulator, development server, or seed service. Restrict database credentials
  to the app database. The API runs migrations at startup.
- The upload volume is private and writable only by the API UID 10001. Do not
  serve it through the proxy or a public bucket. Back it up with the database
  and test restoring both. Authenticated API routes control uploaded-file
  access.
- Keep the Chromium sandbox and the checked-in seccomp profile. See the
  [image requirements](../README.md#production-image) for host AppArmor and
  volume ownership. The frontend runs as the `node` user.
- Configure Firebase web authorized domains for the chosen hostname and enable
  the intended signup providers. Registration is open; verified email and admin
  approval control club access. Bootstrap the initial admin privately using
  the [documented command](../README.md#initial-administrator).

The final images set `SAILOR_ENV=production`. The API rejects emulator auth,
the default or a `demo-` Firebase project, non-HTTPS CORS origins, and disabled
Chromium sandboxing. Frontend startup rejects emulator URLs (including a baked
Vite setting), missing/default Firebase configuration, and unsafe API URLs.
Local `docker-compose.yml` explicitly selects `development`; local commands
default to that mode. Never select development mode on a public host.

## Firebase runtime identity

The API uses Application Default Credentials for the revocation lookup. Prefer
an attached workload identity or workload identity federation. The Compose
example mounts an external-account ADC configuration read-only; its referenced
credential source must also be available to the container. On a platform with
attached identity, omit the file secret and `GOOGLE_APPLICATION_CREDENTIALS`
and let ADC discover that identity. Do not bake service-account private keys
into either image.

Grant the runtime identity `firebaseauth.users.get` on the production project
using a custom role. It reads account status; it does not need account mutation
permissions. Keep the operator identity used to revoke or disable accounts
separate. Firebase web configuration belongs only in `PUBLIC_*` settings;
runtime server credentials must never use that prefix. See
[Admin SDK setup](https://firebase.google.com/docs/admin/setup) and
[Firebase IAM permissions](https://firebase.google.com/docs/projects/iam/permissions).

## Disable access or revoke sessions

1. For immediate club access removal, suspend the account from the members
   screen. Demote an admin first; the app preserves the last active admin.
   Application suspension blocks the next authenticated request, including
   profile and enrollment writes.
2. In Firebase Authentication, disable the account to prevent further sign-in.
   For compromised sessions, run `/app/revoke-sessions` privately with an
   operator ADC identity and the exact `SAILOR_REVOKE_FIREBASE_UID` and
   `SAILOR_FIREBASE_PROJECT_ID`. Local use:
   `cd backend && mise exec -- go run ./cmd/revoke-sessions`. Do both for a
   compromised account. The operator needs `firebaseauth.users.get` and
   `firebaseauth.users.update`; keep those credentials out of the running API.
3. Keep a retained pre-action ID token and confirm a private API request returns
   401. A valid account must still succeed. Record this check for the actual
   deployment in #690.

Every authenticated request uses `VerifyIDTokenAndCheckRevoked`, with a
5-second deadline covering credential refresh and account lookup, and no
application cache of account status. Disabling an account rejects the next
lookup for members and admins. Application suspension provides the
database-controlled cutoff; requests already authorized may finish.

Firebase's SDK accepts a token issued in the same UTC second as a single
`RevokeRefreshTokens` call. Use `revoke-sessions` to revoke, wait for the next
second, revoke again, and confirm the later cutoff. This rejects tokens held
when the command started without rejecting new accounts' first sign-in. Keep
the operator clock synchronized. The extra boundary wait is less than one
second; the whole command has a 15-second deadline and exits unsuccessfully
unless Firebase confirms the later cutoff. Keep the account suspended/disabled
on any failure. A standalone SDK revoke call has the same-second exception
and is not this app's incident procedure.

After the successful operator command's cutoff is visible to Firebase lookup,
the application adds zero seconds of acceptance: the next request rejects a
retained token. Firebase propagation time is external and must be measured at
deployment; no fixed end-to-end propagation guarantee is claimed.

Invalid, expired, revoked, disabled, or deleted sessions receive 401 and the
client signs out. Provider/credential failures receive 503 without granting
access or signing the client out; retry after restoring the provider. The token
enrollment preview remains anonymous on a verification failure and receives no
authenticated privileges. Follow Firebase's
[session management procedure](https://firebase.google.com/docs/auth/admin/manage-sessions).

Local tests verify signed JWTs through the production SDK with local TLS RPC
responses for valid, disabled, revoked, and unavailable states. Emulator tests
are separate: the emulator accepts unsigned tokens and must never be enabled
in production ([Firebase emulator guidance](https://firebase.google.com/docs/emulator-suite/connect_auth)).
