# Account access policy

Approved by the owner on 2026-11-14 for the first public launch. Implementation and deployment verification remain tracked by the security audit issues.

Anyone may register. Reading club data requires a verified Firebase email and explicit administrator approval. Approval is independent of the account role and club membership (`czlonek`, `sympatyk`, `gosc`). Completing onboarding does not approve an account.

No real users exist yet. Preserve existing administrators and migrate all other accounts to pending approval. Administrators still need verified email for club access. Suspension denies authenticated application access, including existing tokens.

| Caller | Own profile/onboarding | Public token preview | Token enrollment as self | Club records | Emergency contacts | Voyage opinions |
| --- | --- | --- | --- | --- | --- | --- |
| Anonymous | No | Yes | No | No | No | No |
| Unverified or pending | Yes | Yes | Yes | No | No | No |
| Approved with verified email | Yes | Yes | Yes | Yes | No | Own opinions only |
| Approved assigned captain | Yes | Yes | Yes | Yes | Assigned crew only | Own opinions and assigned voyage opinions |
| Approved administrator | Yes | Yes | Yes | Yes | Yes | Yes |
| Suspended | No | Anonymous preview only | No | No | No | No |

Club records include directory contact and patent details, trainings, cruise/trip/voyage data, group-chat links, and sailing summaries. Training records remain private to their owner. A pending account cannot obtain a group-chat URL by authenticating to the public enrollment preview. Anonymous previews retain their existing limited fields.

## Hidden drafts

Cruises, trips and voyages start as hidden drafts until an administrator publishes them. A trip or voyage is visible only while it and its cruise are both published. Administrators and the captain group (`users.is_captain`) read hidden records; every other caller, including an approved member and an assigned captain outside the group, gets 404 for a hidden record and its nested routes, and never sees it in a list, count, series-edition list or sea-time total. Share-link previews, share-link enrollment and sign-ups refuse hidden records for every caller. Only administrators publish or unpublish.

An assigned captain is the account linked to the trip or voyage's captain person. Captain-group membership alone grants no access to another voyage. Emergency-contact access covers people assigned to a leg or voyage captained by the caller. Opinion access uses the subject person's linked account or the voyage's assigned captain; listing and downloading enforce the same rule.

Administrators approve accounts through an explicit account approval control. Changing club membership or linking a person does not approve an account. The initial administrator is provisioned privately for an explicitly selected Firebase UID; normal API requests never promote the first caller. Existing role changes and last-admin protection remain in force.

Unverified and pending users can finish their profile and use token-based enrollment. The UI must explain verification and approval states and provide email-verification and sign-out actions. Denied club requests return 403 with a stable reason that the UI can display. Unauthenticated requests return 401. Suspension/revocation must invalidate existing access within the documented maximum window; the implementation must fail closed on verification service failure.

## Verification

Use synthetic accounts for anonymous, unverified, pending, approved member, assigned captain, admin, and suspended callers. Test direct API list/detail requests, opinion downloads, token previews, and writes independently of UI visibility. Include an approved captain requesting another captain's crew and voyage, and a captain-group account, a member and a pending account requesting hidden cruises, trips and voyages directly, in lists, through share links and by sign-up.

No public deployment exists. Live TLS, headers, Firebase settings, admin MFA, backup restore, image restrictions, and the deployed access matrix remain unverified until the owner selects a target. Issue #690 stays open until that evidence exists.
