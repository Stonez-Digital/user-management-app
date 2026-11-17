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

The queue model now includes a nullable `lease_id` UUID field for migration compatibility. The PostgreSQL claim function assigns a fresh value with `gen_random_uuid()` in the same row update that changes the worker owner and claim timestamp. The claim-owner role receives UPDATE on this one additional column and no broader table-level UPDATE privilege.

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

1. Parse and validate the compact JWS structure and fixed header.
2. Load the active verification key by `kid` from a private key store.
3. Verify HS256 over the exact compact-JWS signing input using `pgcrypto.hmac`.
4. Validate issuer, audience, required claims, UUID formats, issued-at, not-before, expiry and maximum token lifetime using database time.
5. Lock and inspect the current queue row; compare job, school, initiator, worker and current lease generation. A replaced or expired lease must fail.
6. Insert the assertion ID into a private table with a unique constraint, together with the current backend PID and transaction ID. The insert and all application writes occur in the same transaction.
7. Expose validated tenant/user claims only through narrow `SECURITY DEFINER` accessors that match both `pg_backend_pid()` and `pg_current_xact_id()`. RLS policies must consume these accessors, never raw GUC values.

The context row and assertion-ID reservation are transactional. A committed transaction permanently consumes the assertion ID. If the transaction rolls back, its context and reservation roll back too, allowing a safe retry because no protected writes committed. A token must not authorize a second committed transaction.

A transaction must establish context before touching tenant tables. Missing, invalid, expired, replayed, or lease-mismatched context fails closed. Context rows from completed transactions are inert because their transaction ID cannot match a later transaction; a separately reviewed cleanup mechanism may remove old inert rows.

The verifier must use a row lock that prevents lease replacement until the protected transaction ends. The implementation must explicitly test lock ordering and avoid deadlocks with worker progress updates and claim/reclaim operations.

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
- A unique assertion-ID constraint serializes concurrent attempts to consume the same assertion.
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
