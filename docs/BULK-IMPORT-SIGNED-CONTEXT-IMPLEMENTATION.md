# Signed bulk-import tenant context: implementation contract

## Status

Implementation design for the draft signed-context work. This document is not authorization to provision production keys, install database objects, alter live roles, or deploy a worker.

The implementation branch is stacked on PR #361. PR #361 and all dependent security PRs remain draft and unmerged.

## 1. Threat model and boundary

The assertion protects the processing database login from being able to invent tenant identity by issuing arbitrary SQL and setting custom PostgreSQL GUC values. The processing login must not be able to read signing keys, invoke a signing operation, write verifier state, or bypass the verifier.

The trusted computing base includes:
- the worker process that receives the queue-claim result and obtains signing capability;
- the Vault identity and versioned secret;
- the PostgreSQL verifier function owner and protected verifier tables;
- PostgreSQL itself and database administrators.

A compromise of the trusted worker process or its Vault signing credential can mint assertions. HMAC does not protect against that compromise. This residual risk must be accepted explicitly; it is not a reason to expose the key to the processing SQL login.

## 2. Cryptographic contract

Use HMAC-SHA256 with a randomly generated key of at least 256 bits. Do not derive the key from a password or application secret.

The signed UTF-8 payload is a newline-delimited, versioned sequence. Every field has a strict grammar (canonical lowercase UUID, decimal unsigned generation, decimal Unix-second timestamps, and a restricted key ID), so newline injection and ambiguous serialization are rejected.

Exact field order:

1. `stonez-bulk-import-context-v1`
2. key ID
3. assertion ID (UUID)
4. job ID (UUID)
5. school ID (UUID)
6. initiating user ID (UUID)
7. PostgreSQL backend PID (positive base-10 integer)
8. PostgreSQL transaction ID (positive canonical base-10 xid8 value)
9. lease generation (positive base-10 integer)
10. issued-at (base-10 Unix seconds)
11. expires-at (base-10 Unix seconds)

The signature is the 32-byte HMAC encoded using unpadded base64url. Go and PostgreSQL must reconstruct identical bytes. Do not sign JSON serialization or locale-dependent timestamps.

Assertions are minted per database transaction, not once per job. Their TTL must be a whole number of seconds, positive and bounded by a small configured maximum. The verifier rejects future-issued assertions beyond 30 seconds of clock skew, expired assertions, invalid lifetimes, malformed signatures and unknown key IDs.

## 3. Lease generation

Add a monotonically increasing `lease_generation BIGINT NOT NULL DEFAULT 0` to `bulk_import_jobs`. The claim function increments it atomically every time a job is claimed or reclaimed and returns the new value. It is separate from `attempts`, which remains an operational retry counter.

The generation must never be reset when a job completes or fails. A new claim always gets a larger generation. The verifier compares the assertion's generation, job, school, initiator, worker ID, running status and lease timestamp against the locked current queue row.

The signer first reads `pg_backend_pid()` and `pg_current_xact_id()` from the active transaction and includes both values in the HMAC. The verifier requires them to match the current backend and transaction before checking the job. This prevents transferring an intercepted assertion to another session or transaction. The verifier then locks the job row for the duration of the processing transaction. A concurrent reclaimer therefore cannot replace the lease while that transaction is using the context.

## 4. Key custody and rotation

Vault is the source of truth for versioned HMAC keys. The worker's Vault identity may read **only the active signing key** and must not have permission to write or rotate keys. A separate operator identity runs the key-provisioning command. Retiring verifier keys are not returned to the worker.

The worker reads a Vault KV v2 secret with this data shape:

```json
{
  "active_key_id": "bulkctx-2026-11",
  "active_key": "<base64-encoded-random-key-of-at-least-32-bytes>"
}
```

The worker uses a Vault Agent token file and these non-secret configuration values: `BULK_IMPORT_CONTEXT_VAULT_ADDR`, `BULK_IMPORT_CONTEXT_VAULT_TOKEN_FILE`, `BULK_IMPORT_CONTEXT_VAULT_KV_MOUNT`, and `BULK_IMPORT_CONTEXT_VAULT_KV_PATH`. The active key is cached in memory for 30 seconds by the worker. The token is never logged, and the HTTP client refuses redirects.

PostgreSQL needs the same HMAC key to verify signatures. Provision a copy into a private, non-exposed `stonez_internal` key table through a separately reviewed operator procedure. The table is owned by a NOLOGIN verifier-owner role. Only the SECURITY DEFINER verifier owner can read it; the processing login, claim login, API runtime, PUBLIC, `anon`, and `authenticated` cannot read it. Do not put key bytes in source control, migrations, SQL literals, command-line arguments, CI logs, application logs, or ordinary environment variables.

The database key table is a verifier-only replica of Vault key material, not a second source of truth. Key IDs are immutable and unique. The operator-only `cmd/bulk-context-key-provisioner` reads the active key from Vault and inserts it into PostgreSQL using a parameterized statement. It requires `BULK_IMPORT_CONTEXT_PROVISIONER_DATABASE_URL` with TLS enabled and the same four Vault configuration variables used by the worker. It refuses to overwrite an existing key ID if the bytes differ and never prints key material. It is not part of worker startup and must not be run automatically during deployment.

Rotation order:
1. Create a new random key and key ID in Vault.
2. Provision the matching key ID and bytes into the private database table using the approved secret-delivery procedure.
3. Verify the database can verify a test signature without returning key material.
4. Switch the worker's active signing key ID.
5. Retain the prior database key for at least maximum assertion TTL plus allowed clock skew, the 30-second worker key cache, and deployment overlap.
6. Remove the old key only after no valid assertion can reference it.

The database key-provisioning procedure must not run automatically at application startup. Key rotation and production provisioning require separate approval.

## 5. PostgreSQL verification and transaction binding

Use `pgcrypto.hmac(data, key, 'sha256')` only after a capability preflight confirms the extension and exact function signature exist in the target PostgreSQL environment. Do not assume the extension is installed and do not enable it in production as part of this implementation. The isolated PostgreSQL 17 CI fixture may install pgcrypto in its disposable database.

The verifier is a narrowly scoped SECURITY DEFINER function with a fixed `pg_catalog` search path and fully schema-qualified references. It:
1. Validates all input formats and the key ID.
2. Loads the matching key inside the function.
3. Reconstructs the canonical payload and verifies the HMAC.
4. Locks the matching queue row and checks the current job/tenant/initiator/worker/generation/status/lease tuple.
5. Rejects expired, stale, mismatched or already-consumed assertions.
6. Inserts assertion ID plus backend PID and current transaction ID into protected verifier state.
7. Sets a transaction-local context pointer only after successful verification.

RLS must not trust the pointer by itself. A SECURITY DEFINER context helper validates that the pointer refers to a verifier-created row for the current backend PID and current transaction ID, and that its expiry and lease remain valid. Policies consume only this verified context. Directly setting the GUC cannot create the protected verifier row.

Assertion consumption and the verified-context row are written in the same transaction as application data. Commit persists consumption and prevents replay. Rollback removes both the context and consumption record, so retrying an assertion after a fully rolled-back transaction cannot authorize any committed work. The verifier must reject a second, different context in the same transaction.

Use a fixed-work byte comparison in the verifier rather than an early-exit comparison of signature bytes. Do not return the key or expected MAC from any function.

### Operator key provisioning

After the role preparation and verifier installation scripts have been reviewed and run in an isolated staging database, provision the Vault active key with:

```bash
go run ./cmd/bulk-context-key-provisioner
```

Required non-secret configuration: `BULK_IMPORT_CONTEXT_VAULT_ADDR`, `BULK_IMPORT_CONTEXT_VAULT_TOKEN_FILE`, `BULK_IMPORT_CONTEXT_VAULT_KV_MOUNT`, and `BULK_IMPORT_CONTEXT_VAULT_KV_PATH`. The operator-only database URL is `BULK_IMPORT_CONTEXT_PROVISIONER_DATABASE_URL` and must use PostgreSQL TLS. This command has not been run against any live database.

## Worker connection configuration

The dedicated worker must receive two separate PostgreSQL URLs:

- `BULK_IMPORT_PROCESSING_DATABASE_URL`: the dedicated `stonez_bulk_process_login` connection used only for tenant-scoped processing transactions. Startup inspects this connection and requires the restricted processing-group membership, no `BYPASSRLS`, no privileged role membership, and no ability to `SET ROLE` into the processing group.
- `BULK_IMPORT_CLAIM_DATABASE_URL`: the separate claim-only connection used only for the narrowly granted queue claim function. Startup validates its claim-role boundary independently.

Both values are mandatory. The worker does not fall back to `DATABASE_URL`, `DB_HOST`/user/password settings, or a shared API connection. The API's `DATABASE_URL` is not read by the worker executable. The two URLs must be configured as distinct credentials; role validation is the authoritative check of their database identities and capabilities.

This change only defines application configuration and startup validation. It does not create either login, grant database privileges, set Render environment variables, or switch any deployed worker. Provisioning and deployment require separate reviewed staging evidence and explicit approval.

## 6. Go integration contract

The Go signer accepts only a `Binding` built from the trusted row returned by the claim function. The worker Vault policy returns only the active signing key; the worker never retrieves retiring verification keys. It must not accept tenant IDs from imported spreadsheet values. The Vault adapter implements the key-source interface and returns only the active key to the signer.

The worker opens a PostgreSQL transaction, reads the backend PID and transaction ID, signs the trusted claim binding including those values, calls the verifier, and then performs tenant reads/writes using that same transaction handle. It must not set `app.current_school_id` or `app.current_user_id` as an authorization mechanism. SQLite-only unit tests may retain their local test behavior but are not evidence for PostgreSQL security.

## 7. Required PostgreSQL tests

Use a disposable PostgreSQL 17 database with actual restricted LOGIN roles. CI must fail or explicitly report a skip when the required role-creation capability is absent; it must never silently substitute SQLite for these tests.

Required cases:
- valid current assertion accepted;
- tampered signature and each tampered claim rejected;
- unknown key ID rejected;
- expired and excessively long assertions rejected;
- missing, malformed and future-issued assertions rejected;
- assertion replay after commit rejected;
- rollback does not leave a consumed assertion or verified context;
- second context in one transaction rejected;
- replaced lease generation and changed worker ID rejected;
- expired/reclaimed lease rejected;
- cross-school reads and writes denied;
- processing login cannot read key material or write verifier state;
- manually setting the context GUC without verifier state grants no access;
- pooled connection reuse after commit/rollback has no residual authority;
- concurrent reclaim cannot replace a lease while a verified processing transaction holds its row lock.

## 8. Explicit exclusions

This implementation does not install pgcrypto in a live database, provision Vault or database keys, create production roles, change production secrets, change Render/Supabase configuration, deploy a worker, enable restricted-role environment flags, merge PRs, or modify production data. Those actions require separate reviewed and explicit approval.


## CI isolation

The signed-context PostgreSQL integration test creates unique schemas and restricted roles in the disposable `TEST_DATABASE_URL` database. The separate queue-role SQL-script fixture runs in its own `bulk_queue_scripts_test` database so its global `stonez_internal` schema and role setup cannot collide with authentication bootstrap or application integration tests.
