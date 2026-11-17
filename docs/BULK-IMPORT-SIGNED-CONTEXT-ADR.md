# Bulk-import signed tenant context — architecture decision

## Status

**Implementation decision for review; no database installation or deployment is authorized.**

This decision resolves the cryptographic format and trust boundaries needed by the processing-context verifier. It is intentionally separate from the existing design proposal so reviewers can assess the cryptographic choice before database privileges or RLS policies are changed.

## Decision summary

Use a compact, short-lived JWT signed with **HMAC-SHA-256 (HS256)** by the trusted Go bulk-import worker. PostgreSQL verifies the exact compact-JWS signing input with `pgcrypto.hmac` and a database-held verification key that is inaccessible to the processing login.

The processing login receives only EXECUTE on narrowly scoped context functions. It never receives SELECT on the key store, direct access to verified-context records, or access to any signing operation.

The raw token is passed as a function argument to the verifier. It is never placed in a PostgreSQL GUC, logged, returned from SQL, or stored in a job row.

## Why HS256 / compact JWS

- Go already has the maintained `github.com/golang-jwt/jwt/v5` dependency.
- PostgreSQL's `pgcrypto` provides HMAC-SHA-256, avoiding a custom cryptographic implementation in SQL.
- Compact JWS verification authenticates the exact bytes `base64url(header) + "." + base64url(payload)`; PostgreSQL does not need to reproduce Go JSON serialization.
- A `kid` header supports controlled key rotation.
- HMAC is appropriate here because both the trusted worker and verifier are controlled components. It does mean compromise of either copy of the shared key permits assertion forgery; this residual risk is explicit.

Do not substitute an ad-hoc JSON signature, plain SHA-256, an unsigned GUC, or a key embedded in a SQL function definition.

## Claim-generation implementation status

The queue model now includes a nullable `lease_id` UUID field for migration compatibility. Additive application migration 48 adds the PostgreSQL column and index; it is separate from migration 47 because deployed databases may already have recorded version 47 and will not rerun it. The PostgreSQL claim function assigns a fresh value with `gen_random_uuid()` in the same row update that changes the worker owner and claim timestamp. The claim-owner role receives UPDATE on this one additional column and no broader table-level UPDATE privilege.

The Go SQLite test path mirrors this by assigning a fresh UUID on every claim. A worker transaction must present the lease generation returned with its claimed job; the generation is included in the locked queue-row predicate. A missing generation or replaced generation is rejected before the transaction callback can touch tenant data.

Existing rows may have a NULL lease generation until they are claimed again. The signed-context verifier must reject NULL generations and must never mint or infer a generation from worker input.

## Signed assertion contract

The token must use a fixed algorithm allowlist (`HS256`) and a fixed issuer and audience. The verifier must reject algorithm substitution, unknown `kid`, unexpected issuer/audience, malformed UUIDs, missing claims, duplicate JSON keys where detectable, and tokens outside the permitted time window.

Required claims:

| Claim | Meaning |
|---|---|
| `iss` | Fixed issuer identifying the Stonez bulk-import worker |
| `aud` | Fixed audience identifying the PostgreSQL bulk-import context verifier |
| `sub` | Worker identity |
| `jti` | Cryptographically random, unique assertion ID |
| `iat` | Issued-at time |
| `nbf` | Not-before time |
| `exp` | Expiry time |
| `job_id` | Claimed bulk-import job |
| `school_id` | Tenant/school |
| `initiated_by` | User who initiated the job |
| `worker_id` | Worker UUID matching the claim |
| `lease_id` | Fresh, unguessable lease generation assigned on every successful claim |

All identity and lease claims must be derived from the row returned by the restricted claim function, never from spreadsheet rows, request payloads, or caller-supplied tenant headers.

The Go signer must require a key ID and at least 32 bytes of key material. The token lifetime should be short (initial target: 30 seconds, maximum accepted lifetime: 60 seconds). PostgreSQL's clock is authoritative for acceptance; allow only a small, explicit clock-skew tolerance. The signer must generate a new `jti` for every processing transaction.

## Database verification and transaction binding

The verifier is a narrowly scoped `SECURITY DEFINER` function with a fixed `pg_catalog` search path and fully schema-qualified object references.

1. Parse exactly three non-empty compact-JWS segments; require canonical unpadded base64url, an object header/payload, the fixed `HS256` algorithm and a known `kid`. Reject duplicate JSON keys, unsupported critical headers and unexpected header members.
2. Load the active verification key by `kid` from a private key store, holding a shared lock so retirement cannot race an in-flight verification.
3. Verify HS256 over the exact compact-JWS signing input using the discovered, schema-qualified `pgcrypto.hmac` function. Compare the fixed 32-byte signatures in constant time.
4. Validate issuer, audience, subject, required claims, canonical UUID formats, issued-at, not-before, expiry, clock skew and maximum token lifetime using database time.
5. Lock and inspect the current queue row; only after acquiring the row lock, read `clock_timestamp()` and compare job, school, initiator, worker, lease generation and lease expiry. A replaced or expired lease must fail.
6. Insert the assertion ID into a private table with a unique constraint, together with the current backend PID and transaction ID. The insert and all application writes occur in the same transaction.
7. Expose validated tenant/user claims only through narrow `SECURITY DEFINER` accessors that match both `pg_backend_pid()` and `pg_current_xact_id()`. RLS policies used by the bulk-worker processing role must consume these accessors, never raw GUC values.

The context row and assertion-ID reservation are transactional. A committed transaction permanently consumes the assertion ID. If the transaction rolls back, its context and reservation roll back too, allowing a safe retry because no protected writes committed. A token must not authorize a second committed transaction.

A transaction must establish context before touching tenant tables. Missing, invalid, expired, replayed, or lease-mismatched context fails closed. Context rows from completed transactions are inert because their transaction ID cannot match a later transaction; a separately reviewed cleanup mechanism may remove old inert rows.

The verifier must use a row lock that prevents lease replacement until the protected transaction ends. The implementation must explicitly test lock ordering and avoid deadlocks with worker progress updates and claim/reclaim operations.

## User-role insertion ceiling

The worker's `users` INSERT privilege is column-scoped to the fields emitted
by the bulk-import model. A separate restrictive RLS policy permits only the
`student`, `teacher`, and `parent` roles. Because the policy is restrictive,
it is ANDed with permissive tenant policies and remains an independent role
ceiling even if the tenant-scoping INSERT policy is later weakened.

The immutable `stonez_context_private.bulk_worker_user_role_allowed(text)`
predicate is checked by the startup readiness gate, including its function
body and the deparsed restrictive-policy expression. Readiness also rejects
table-level INSERT on `public.users` and requires the approved column-level
INSERT grants. PostgreSQL 17 adversarial tests prove that student insertion
works, privileged-role insertion fails, and readiness rejects a weakened
role-policy expression.

## Processing-role separation

The worker's processing connection must use a dedicated non-privileged login/group role, distinct from the normal API runtime login and from the queue-claim login. The existing worker entrypoint already accepts separate processing and claim database URLs; the processing URL must be provisioned for this worker-only role before verifier-backed RLS is enabled.

- The worker processing login must not be a member of the API runtime group, queue-owner role, verifier-owner role, or any SUPERUSER/BYPASSRLS role.
- Worker-only RLS policies must derive school and initiator identity from the transaction-bound verified-context accessors.
- The worker must stop setting or relying on `app.current_school_id` and `app.current_user_id` when the signed verifier is integrated.
- Existing request-scoped API tenant context is a separate trust path. Do not remove or rewrite normal API GUC policies as part of this worker-only change unless an independently designed API context boundary is ready. Keep the worker login out of those GUC-authorized API roles so it cannot use the weaker path as a fallback.
- Until verifier-backed worker policies and the restricted processing role are installed and tested, the signed verifier must not be treated as a replacement for the current worker GUC path.

## Key custody and rotation

- Keep the signing key in the worker's secret manager/environment (Render secret configuration), never in source, a Docker image, logs, job payloads, or GitHub Actions output.
- Keep the verification copy in a private database key store owned by a dedicated NOLOGIN verifier-owner role. The processing login cannot read or modify that store.
- Do not store key material in a custom GUC, function source, migration literal, assertion, or application-visible table.
- Provision keys out of band through a parameterized, audited operator procedure. The procedure must not echo key bytes or place them in shell history.
- Use a key ID in the token header. Rotation is staged: add a new verification key, deploy the worker signer with the new key ID, observe successful verification, then retire the old key after the maximum token lifetime plus clock-skew allowance.
- Never reuse a key ID for different key material. Key retirement must not invalidate an in-flight transaction that has already verified its assertion.
- CI uses disposable random test keys only.

The exact production secret-manager and database provisioning mechanism must be confirmed for the target Supabase project before any staging installation. This ADR does not assume that an extension or Vault feature is already enabled.

## Replay, lease replacement and concurrency

- Each successful queue claim must assign a fresh `lease_id`; reclaiming a job must replace it atomically.
- The assertion is bound to that lease ID, not merely to job ID or worker ID.
- The verifier locks the job row and validates the current lease and expiry.
- A unique assertion-ID constraint serializes concurrent attempts to consume the same assertion. A key-admin-only pruning function removes committed context rows older than ten minutes; this exceeds the maximum assertion lifetime plus clock-skew allowance, while uncommitted rows remain invisible to cleanup.
- A lease replacement cannot occur while a valid processing transaction holds the verifier's job-row lock; after replacement, old assertions fail.
- The worker must stop processing and must not update progress after losing its lease.

## Required PostgreSQL 17 tests

Tests must run against disposable PostgreSQL 17 and actual restricted logins, not SQLite or mocked role checks.

- Valid token accepted; valid claims are available only in the same transaction.
- Wrong job, school, initiator, worker, lease ID, issuer or audience rejected.
- Tampered payload, signature, header, unknown key ID and unsupported algorithm rejected.
- Missing, malformed, not-yet-valid, expired and overlong tokens rejected.
- Reusing a committed assertion ID rejected; rollback permits retry but commits no protected data.
- Two concurrent uses of one assertion cannot both commit.
- Lease replacement invalidates an old assertion.
- Processing login cannot read keys, read context rows, change keys, or call a signing function.
- Arbitrary SQL setting tenant GUCs grants no tenant access.
- Context does not survive commit, rollback, or pooled-connection reuse.
- Lock ordering does not deadlock claim, processing and progress paths.
- School A cannot read, write, or reference School B data.

The test job must fail, not silently pass, if PostgreSQL role creation or required extensions are unavailable.

## Rollout constraints

This work may add Go code, SQL source files, and disposable PostgreSQL integration tests on an isolated branch. It must not execute SQL against Supabase, provision live roles, install extensions, modify Render secrets, alter production credentials, deploy the worker, or enable a production guard.

Before staging installation, separately verify extension availability, key provisioning, table/column privileges, all RLS policies, worker SQL side effects, and the restricted role membership graph. Production cutover requires a separate explicit approval after staging evidence is reviewed.


## Key rotation procedure

1. Generate a new independent 256-bit key outside the database and store it in the worker secret manager.
2. Register the new key ID and raw key bytes through a parameterized call to `stonez_context_private.register_verification_key`; the key-admin function records the actor and action without recording key material.
3. Deploy the dedicated worker with the new key ID and matching base64url key. During the overlap, both old and new keys remain valid.
4. Confirm all worker instances use the new key, then call `retire_verification_key` for the old key with an effective timestamp.
5. For suspected compromise, call `revoke_verification_key` immediately. Revocation blocks new assertions signed with that key; already verified transactions hold a shared key-row lock until they finish.
6. Run the key-admin-only replay-ledger pruning command periodically. Never grant the processing login access to key-management functions or key bytes.

Do not put key material in shell history, source control, issue comments, CI logs, or SQL text logs. Use a parameterized client and a secret manager for registration and worker configuration.

## Implementation status — draft branch only

The first PostgreSQL verifier slice is implemented as operator-run SQL, with no automatic application migration:

- `scripts/security/install_bulk_import_context_verifier.sql` creates the private key store, transaction-context/replay table, strict HS256 verifier, transaction accessors, and key register/retire/revoke functions. Key-management actions are recorded without recording key bytes.
- `scripts/security/install_bulk_import_context_worker_policies.sql` installs worker-only queue and bulk-import data policies. It is run separately as the owner of the application tables; the ordinary API tenant policies are not rewritten.
- `scripts/security/provision_bulk_import_processing_login.psql` provisions a dedicated processing login. The `stonez_bulk_worker_processing` NOLOGIN group is separate from the existing `stonez_bulk_worker` claim login, which retains its distinct API-runtime/queue-claim responsibilities.
- `scripts/security/prune_bulk_import_context_ledger.sql` runs the key-admin-only cleanup function. Schedule it periodically (hourly is sufficient for the ten-minute replay-ledger retention window).
- The Go worker has an opt-in signed-context path. It is enabled only when `BULK_IMPORT_SIGNED_CONTEXT_REQUIRED=true`, with `BULK_IMPORT_CONTEXT_SIGNING_KEY_ID` and `BULK_IMPORT_CONTEXT_SIGNING_KEY` supplied by the worker secret manager. Startup validates that the processing login is not privileged and is not a member of API, queue-owner, verifier-owner, or key-admin roles.
- The default remains the existing worker path. Do not enable the flag until the separate processing login, verifier key, and every required worker RLS policy have been installed and verified in staging.

A dedicated PostgreSQL 17 workflow runs the verifier against a disposable database with pgcrypto installed in a non-default schema. Its test harness covers signature validation, transaction binding, rollback retry, replay, expiry, key selection, row-lock fencing, cross-tenant RLS, and role separation. A green workflow is evidence for this verifier slice only; it is not approval to install it in Supabase or change Render credentials.

