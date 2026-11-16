# Account access policy

Approved by the owner on 2026-11-14 for the first public launch. Implementation and deployment verification remain tracked by the security audit issues.

Policy change, 2026-11-15: the owner replaced explicit administrator approval with email verification. A verified email is now enough to use the app; nobody from the club approves new accounts, and the club keeps the ability to suspend an account. Migration `035_drop_account_approval.sql` drops `users.access_approved`.

Anyone may register. Reading club data requires a verified Firebase email and an account that is not suspended. Access is independent of the account role and club membership (`czlonek`, `sympatyk`, `gosc`). Administrators also need a verified email for club access. Suspension denies authenticated application access, including existing tokens.

| Caller | Own profile/onboarding | Public token preview | Token enrollment as self | Club records | Yacht list | Emergency contacts | Voyage opinions | Leg completion |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Anonymous | No | Yes | No | No | No | No | No | No |
| Unverified email | Yes | Yes | Yes | No | No | No | No | No |
| Verified email | Yes | Yes | Yes | Yes | No | No | Own opinions only | No |
| Verified assigned captain | Yes | Yes | Yes | Yes | Through the assigned voyage only | Assigned crew only | Own opinions; read, generate and delete assigned voyage opinions | Own legs after the disembark date |
| Verified captain group | Yes | Yes | Yes | Yes | Yes | As an assigned captain only | Own opinions; assigned voyage opinions as its captain | As an assigned captain only |
| Verified administrator | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Any planned leg |
| Suspended | No | Anonymous preview only | No | No | No | No | No | No |

Club records include directory contact and patent details, trainings, cruise/trip/voyage data, group-chat links, and sailing summaries. A leg or voyage names its yacht (name and type) for every reader of it. Training records remain private to their owner. An unverified account cannot obtain a group-chat URL by authenticating to the public enrollment preview. Anonymous previews retain their existing limited fields.

## Yacht list

Policy change, 2026-11-15 (issue #737): the club yacht list (Jachty) is a working tool for the board and the captains. Only administrators and the captain group (`users.is_captain`) read `GET /yachts` and `GET /yachts/{id}` and see the Jachty page; every other caller gets 403 and the page sends them to the dashboard. Only administrators change the list directly. A voyage's assigned captain outside the group reads the list only through that voyage (`GET /voyages/{id}/yachts`) to pick the yacht in the voyage log, and may add a yacht from it (`POST /voyages/{id}/yachts`).

## Hidden drafts

Cruises, trips and voyages start as hidden drafts until an administrator publishes them. A trip or voyage is visible only while it and its cruise are both published. Administrators and the captain group (`users.is_captain`) read hidden records; every other caller, including a verified member and an assigned captain outside the group, gets 404 for a hidden record and its nested routes, and never sees it in a list, count, series-edition list or sea-time total. Hiding a record does not change who may edit it: a voyage's assigned captain still writes its log and ports while it is hidden. A share-link preview lists the caller's leg enrollments only for the published legs it offers. Share-link previews, share-link enrollment and sign-ups refuse hidden records for every caller. Only administrators publish or unpublish.

An assigned captain is the account linked to the trip or voyage's captain person. Captain-group membership alone grants no access to another voyage. Emergency-contact access covers people assigned to a leg or voyage captained by the caller. A leg's crew list carries each person's phone and emergency contact only for administrators and that leg's captain; the captain of another leg the same person sails does not get them there. The leg's captain sees that crew read-only and cannot change it. Opinion access uses the subject person's linked account or the voyage's assigned captain; listing and downloading enforce the same rule, and the voyage's assigned captain may also generate and delete its opinions. Every voyage captain write (log, yacht added from the voyage, ports, opinions) checks the captain in the write statement itself, without locking the voyage, so a captain reassigned and committed before that statement starts gets 403 (404 for a hidden voyage) and changes nothing. A reassignment that commits while the statement runs does not stop it; only the log update waits for an open reassignment of its voyage row and then checks the new captain, and a port reorder checks again in each position write. An opinion file is written before its row and removed again if the write is refused or fails; a replaced or deleted opinion's file is removed only after the row changed. A leg's assigned captain may complete it into a voyage only once its disembark date is before today's date in Europe/Warsaw; the API checks the rule again inside the completion transaction, and refuses with 403 before that date, for a leg without a disembark date and for every other non-administrator.

Administrators suspend and restore accounts through an account access control. Changing club membership or linking a person does not change suspension. An administrator must be demoted before suspension, so the last administrator cannot be suspended, and a suspended account cannot be promoted. The initial administrator is provisioned privately for an explicitly selected Firebase UID; normal API requests never promote the first caller. Existing role changes and last-admin protection remain in force.

Unverified users can finish their profile and use token-based enrollment. The UI must explain the verification and suspension states and provide email-verification and sign-out actions. Denied club requests return 403 with a stable reason that the UI can display. Unauthenticated requests return 401. Suspension/revocation must invalidate existing access within the documented maximum window; the implementation must fail closed on verification service failure.

## Verification

Use synthetic accounts for anonymous, unverified, verified member, assigned captain, admin, and suspended callers. A new account with a verified email and no administrator action must read club data. Test direct API list/detail requests, opinion downloads, token previews, and writes independently of UI visibility. Include a member, a captain-group account and an assigned captain outside the group requesting the yacht list directly and through the voyage, a verified captain requesting another captain's crew and voyage, and a captain-group account, a member and an unverified account requesting hidden cruises, trips and voyages directly, in lists, through share links and by sign-up.

No public deployment exists. Live TLS, headers, Firebase settings, admin MFA, backup restore, image restrictions, and the deployed access matrix remain unverified until the owner selects a target. Issue #690 stays open until that evidence exists.
