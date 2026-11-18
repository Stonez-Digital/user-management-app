# Signed bulk-import context — staging operations runbook

## Status and safety boundary

This is an operator procedure for a **separate, disposable or dedicated staging database**. It is not authorization to install anything in production. The verifier, worker policies, processing login, and outbox are operator-installed; application startup does not install them.

Before starting:

- Confirm the Supabase project ref and database branch are dedicated to this application’s staging environment and are separate from production.
- Confirm a recent, restorable backup and an approved maintenance window.
- Use a database operator account with only the privileges needed by each script. Do not use the application API or worker credentials.
- Confirm PostgreSQL 17 and the `pgcrypto.hmac(bytea,bytea,text)` function are available. The verifier discovers the extension schema from PostgreSQL catalogs; do not assume it is `public`.
- Record the starting migration version, role memberships, table owners, RLS state, and grants. Save the read-only audit output with the change record.
- Do not paste connection strings, passwords, signing keys, or reset tokens into GitHub, chat, shell history, or CI logs.

Use a dedicated staging connection only:

```bash
export STAGING_DATABASE_URL='(load from approved secret store; do not echo)'
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 -c \
  "SELECT current_database(), current_user, version();"
```

Stop if the database identity is unexpected, if the environment cannot be proven separate from production, or if any preflight fails.

## Installation order

Each script is transactional where indicated, but the entire rollout is not one cross-script transaction. Stop on the first error; do not continue with later steps.

### 1. Apply application migration 48

Apply the reviewed application migration that adds `public.bulk_import_jobs.lease_id` using the repository’s normal migration procedure for this staging environment.

Verify before proceeding:

```sql
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema='public'
  AND table_name='bulk_import_jobs'
  AND column_name='lease_id';
``

Expected: exactly one `lease_id` column of type `uuid`. Queue-role provisioning deliberately refuses to run without it.

### 2. Provision and verify queue roles

Run:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/provision_bulk_import_queue_roles.sql
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/verify_bulk_import_queue_roles.sql
``

Expected: no exception. Preserve the verifier output. The queue owner must be NOLOGIN/NOINHERIT; the API runtime and queue-claim groups must remain distinct; the queue-claim group must not receive direct public-table or sequence privileges.

### 3. Install the queue claim function

Run:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/install_bulk_import_claim_function.sql
``

Then verify the installed function and its grants using the role catalog checks in `verify_bulk_import_queue_roles.sql` and the disposable queue-role regression script. The API runtime must not be able to execute the global claim function.

### 4. Provision or transition the queue-claim login

For a new environment with no existing `stonez_bulk_worker` login:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/provision_bulk_import_worker_login.psql
``

For an existing login that currently inherits `stonez_api_runtime`, use the transactional transition instead:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/migrate_bulk_import_worker_to_queue_claim.psql
``

The transition requires the operator to hold the required ADMIN OPTION memberships. It removes broad API-runtime membership and preserves the existing worker password. Do not run both alternatives. Verify the final login has only the queue-claim group membership and can execute only the intended claim function.

### 5. Install the signed-context verifier

Run as the owner of `public.bulk_import_jobs` (or a database operator explicitly authorized to grant the verifier owner its narrow queue privileges):

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/install_bulk_import_context_verifier.sql
``

The installer requires the queue roles and pgcrypto HMAC function. It creates the private schema and verifier roles/functions. It also installs owner-specific default privileges so future functions created by `stonez_bulk_context_owner` in the private schema are not executable by PUBLIC.

Do not manually copy key bytes into SQL source or migration files.

### 6. Install worker-only RLS policies

Run as the owner of the application tables:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/install_bulk_import_context_worker_policies.sql
``

Review the resulting policy and privilege catalog before continuing. The worker policies must derive tenant scope from the transaction-bound signed-context accessors; the API’s request-scoped GUC policy path is not replaced by this change.

### 7. Provision the dedicated processing login

Run interactively so psql can prompt for the password without placing it in shell history:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/provision_bulk_import_processing_login.psql
``

Store the generated credential only in the approved worker secret manager. The login must inherit the exclusive `stonez_bulk_worker_processing` group, must not be able to SET ROLE into it, and must not be a member of API runtime, queue owner, queue-claim login, verifier owner, or key-admin roles.

### 8. Install the credential-notification outbox

Run after the verifier and worker policies:

```bash
psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f scripts/security/install_bulk_import_account_setup_outbox.sql
``

The processing login must have no direct table access to the private outbox. It receives only the narrow signed-context-bound function grants.

### 9. Register the verifier key out of band

Generate an independent random key of at least 256 bits using the approved secret-management process. Store the signing copy in the worker secret manager and register the verification copy through the parameterized key-admin procedure. Do not put key bytes in command-line arguments, SQL text files, shell history, CI output, or chat.

Use a unique key ID. Verify the key ID, validity interval, and key-admin audit event without selecting or printing the key bytes. Configure the worker’s processing database URL, signing key ID, and base64url signing key from the secret manager. Do not enable the signed-context-required flag yet.

## Read-only verification gate

Before enabling signed-context mode, collect evidence from the staging database and the restricted processing login:

1. PostgreSQL version is 17 and pgcrypto HMAC resolves to the installed extension schema.
2. Queue, API, processing, verifier-owner, and key-admin roles have the intended attributes and membership graph.
3. The processing login is the only member of the processing group; it cannot SET ROLE, administer that group, or inherit API/queue/owner/key-admin roles.
4. The verifier, accessors, eligibility predicate, and outbox functions are owned by `stonez_bulk_context_owner`, are SECURITY DEFINER where required, and have fixed `search_path=pg_catalog`.
5. No function in `stonez_context_private` is executable by PUBLIC. Processing and key-admin function EXECUTE grants match the explicit allowlists; neither role can execute the other role’s functions.
6. The processing login cannot read verifier keys, verified-context rows, or the outbox table directly; it cannot update queue lease-generation columns or access password hashes.
7. Required RLS policies, expressions, and column grants match the startup readiness checks exactly.
8. A real connection using the dedicated processing login passes `ValidateBulkImportProcessingRole` and `ValidateBulkImportSignedContextReady`.
9. The full disposable PostgreSQL 17 verifier workflow passes, including forged-token, replay, expiry, lease-fencing, cross-tenant, outbox crash/retry, and permission tests.

A green CI run proves the disposable fixture only. It is not proof that the live staging catalog matches it. Do not enable the worker flag until both CI and live, read-only catalog verification pass.

## Enablement and observation

Only after a separate review of the evidence:

- Enable signed-context mode on the staging worker only.
- Supply the processing database URL and signing-key variables from the secret manager.
- Start with one staging worker instance and a non-production test import.
- Confirm successful queue claim, transaction-bound context verification, tenant isolation, durable outbox enqueue/delivery, and useful failure telemetry.
- Confirm the normal API request path and ordinary school workflows remain unaffected.
- Keep the old key valid during a controlled rotation overlap. Retire it only after all workers use the new key and the maximum assertion lifetime plus clock-skew allowance has elapsed.

Production rollout requires a separate approval and a separate production change plan. This runbook does not authorize production installation or cutover.

## Failure handling and rollback

- Stop immediately on any failed preflight, unexpected grant, unexpected role membership, missing policy, or startup-readiness failure.
- Do not rerun a partially failed sequence blindly. Capture the exact failing statement, catalog state, and transaction outcome first.
- If signed-context mode was enabled in staging and fails, disable the flag and stop the processing worker. Do not fall back to the API runtime credentials for processing.
- Keep the signing key and database verification key intact while investigating. Revoke a key only for suspected compromise or an approved key-rotation action; revocation is not a generic rollback.
- The scripts are individually transactional, but there is no single automatic rollback script for the complete rollout. Do not use `DROP SCHEMA ... CASCADE`, broad `DROP OWNED`, or ad-hoc role deletion as rollback.
- Revert only the specific staging role memberships or grants that the reviewed recovery plan identifies. Preserve audit records and capture a post-rollback catalog snapshot.
- Restore from the verified staging backup only if the database state cannot be safely reconciled with targeted, reviewed changes.

## Evidence to attach to the change record

- Staging project ref and branch identity (never connection credentials).
- Before/after migration version and `lease_id` column verification.
- Queue role verification output.
- Processing login membership and role-attribute output.
- Function owner, SECURITY DEFINER, fixed search path, and effective EXECUTE ACL report.
- RLS policy and table/column privilege report.
- CI run URLs and commit SHA.
- Restricted-login startup readiness result.
- Test import result, outbox delivery result, and rollback/disablement evidence if exercised.
