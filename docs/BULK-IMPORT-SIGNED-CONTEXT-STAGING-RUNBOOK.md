# Signed bulk-import context — staging operator runbook

## Status and safety boundary

This runbook is for a **new, isolated staging database only**. It is not authorization to install anything in Supabase, Render, or production. The free Supabase organization currently has no designated application staging project, and a new project could not be created because the organization reached its active free-project limit. Do not substitute the populated `user-management-app` project or the unrelated media-recovery project.

The PostgreSQL 17 GitHub Actions job is a disposable integration-test environment. It validates the SQL and role boundary, but it is not a persistent staging database and does not verify deployed Supabase/Render credentials.

Keep PR #363 draft and unmerged. Do not set `BULK_IMPORT_SIGNED_CONTEXT_REQUIRED=true` until every staging gate below is complete and separately approved.

## Required staging properties

- A dedicated, disposable PostgreSQL 17 database with no production data or credentials.
- A database administrator able to create roles, extensions, and test identities.
- The application schema at the migration level required by the installers, including `bulk_import_jobs.lease_id` and `processed_rows_json`.
- `pgcrypto` installed. The verifier installer discovers its schema from extension metadata; do not assume it is in `public`.
- A separate secret manager for the worker signing key. Never put key bytes in SQL files, shell history, CI output, issue comments, or this runbook.
- A rollback snapshot/restore point and a named operator responsible for recording evidence.

## Installation sequence

Run each step only against the explicitly identified staging database. Stop on the first error; do not continue with a partially installed boundary.

1. **Identify and verify the target.** Connect using the staging administrator and record:
   `SELECT current_database(), current_user, version();`
   Confirm the database name and connection host are unmistakably staging. The target must not be a production or shared application database.

2. **Apply application migrations.** Apply the repository's reviewed application migrations through the normal migration mechanism. Confirm the required columns exist:
   `SELECT to_regclass('public.bulk_import_jobs') IS NOT NULL,`
   `       EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='public.bulk_import_jobs'::regclass AND attname='lease_id' AND attnum>0 AND NOT attisdropped),`
   `       EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='public.bulk_import_jobs'::regclass AND attname='processed_rows_json' AND attnum>0 AND NOT attisdropped);`
   All three results must be true.

3. **Install queue-claim prerequisites.** Follow the existing queue-role operator scripts and migration-48 prerequisite documented in `docs/BULK-IMPORT-SIGNED-CONTEXT-ADR.md`. Confirm the claim login is not a member of `stonez_api_runtime` and has no direct application-table or sequence privileges beyond the explicitly reviewed claim path. Do not rotate an existing credential as part of this staging exercise.

4. **Install the verifier.** As the application-table/database operator, run:
   `psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_verifier.sql`
   The script is transactional and must complete without errors. It creates the private owner, key-admin and processing group roles, private schema, verifier functions, and owner-specific default function ACLs.

5. **Install worker RLS policies.** Run as the owner of all application tables named by the installer:
   `psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_worker_policies.sql`
   Do not run as a superuser merely to bypass the ownership preflight. Resolve ownership correctly in the isolated database.

6. **Install the account-setup outbox.** Run after the verifier and worker policy installers:
   `psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_account_setup_outbox.sql`
   Confirm the processing group has no direct outbox-table privileges and only the five scoped outbox function entry points.

7. **Provision the dedicated processing login.** Run interactively so psql's \`\\password\` prompt handles the password without embedding it in command arguments:
   `psql "$STAGING_DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/security/provision_bulk_import_processing_login.psql`
   Store the generated credential directly in the staging worker secret manager. Never paste it into chat, GitHub, CI logs, or a source file. The script deliberately refuses to alter an existing login; if it already exists, stop and investigate rather than editing it in place.

8. **Register a staging-only verification key.** Generate an independent random key of at least 32 bytes using an approved secret manager. Store the worker signing copy in the staging worker secret manager and register the verification copy through the parameterized key-admin procedure. Do not put the key literal in a saved SQL script or shell command. Use a unique key ID and record only that ID, creation time, and operator identity.

9. **Run read-only post-install checks.** Use the queries below and the application's signed-context readiness check. Save redacted output (role names, booleans, privilege results, object names); never save passwords or key bytes.

## Required post-install evidence

### Processing login membership and role attributes

```sql
SELECT r.rolname, r.rolcanlogin, r.rolsuper, r.rolcreatedb,
       r.rolcreaterole, r.rolreplication, r.rolbypassrls, r.rolinherit,
       pg_has_role(r.rolname, 'stonez_bulk_worker_processing', 'USAGE') AS inherits_processing_group,
       pg_has_role(r.rolname, 'stonez_bulk_worker_processing', 'SET') AS can_set_processing_group,
       pg_has_role(r.rolname, 'stonez_api_runtime', 'MEMBER') AS is_api_runtime_member,
       pg_has_role(r.rolname, 'stonez_bulk_queue_owner', 'MEMBER') AS is_queue_owner_member,
       pg_has_role(r.rolname, 'stonez_bulk_worker', 'MEMBER') AS is_claim_login_member,
       pg_has_role(r.rolname, 'stonez_bulk_context_owner', 'MEMBER') AS is_context_owner_member,
       pg_has_role(r.rolname, 'stonez_bulk_context_key_admin', 'MEMBER') AS is_key_admin_member
FROM pg_roles AS r
WHERE r.rolname = 'stonez_bulk_worker_processing_login';
```

Expected: exactly one row; `rolcanlogin` and `rolinherit` true; every privilege flag false; inherits the processing group; cannot SET ROLE into it; every forbidden-membership boolean false.

```sql
SELECT parent.rolname AS granted_role, member.rolname AS member,
       m.admin_option, m.inherit_option, m.set_option
FROM pg_auth_members AS m
JOIN pg_roles AS parent ON parent.oid=m.roleid
JOIN pg_roles AS member ON member.oid=m.member
WHERE parent.rolname='stonez_bulk_worker_processing'
   OR member.rolname='stonez_bulk_worker_processing_login'
ORDER BY 1,2;
```

Expected: exactly one membership row, from the processing login to the processing group, with no admin option, inheritance enabled, and SET disabled. The processing group must have no parent memberships and no other members.

### Effective table privileges

```sql
SELECT has_table_privilege('stonez_bulk_worker_processing_login',
                           'stonez_context_private.verification_keys','SELECT') AS can_read_keys,
       has_table_privilege('stonez_bulk_worker_processing_login',
                           'stonez_context_private.verified_contexts','SELECT') AS can_read_context_rows,
       has_table_privilege('stonez_bulk_worker_processing_login',
                           'stonez_context_private.account_setup_outbox','SELECT') AS can_read_outbox,
       has_table_privilege('stonez_bulk_worker_processing_login',
                           'public.users','UPDATE') AS can_update_users,
       has_table_privilege('stonez_bulk_worker_processing_login',
                           'public.users','DELETE') AS can_delete_users,
       has_table_privilege('stonez_bulk_worker_processing_login',
                           'public.bulk_import_jobs','DELETE') AS can_delete_jobs;
```

Expected: all six results false. Also verify the intended column-level SELECT/INSERT and queue status/progress UPDATE grants through `information_schema.column_privileges` or `has_column_privilege`; table-level privilege checks alone do not prove the approved column inventory.

### Effective function privileges

```sql
SELECT p.oid::regprocedure AS function_name,
       has_function_privilege('stonez_bulk_worker_processing_login',p.oid,'EXECUTE') AS processing_can_execute,
       has_function_privilege('stonez_bulk_context_key_admin',p.oid,'EXECUTE') AS key_admin_can_execute,
       EXISTS (
           SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a
           WHERE a.grantee=0 AND a.privilege_type='EXECUTE'
       ) AS public_can_execute
FROM pg_proc p
JOIN pg_namespace n ON n.oid=p.pronamespace
WHERE n.nspname='stonez_context_private'
ORDER BY 1;
```

Expected: no function executable by PUBLIC; the processing group can execute only the approved verifier/accessor/user-role/outbox functions; the key-admin group can execute only key-management and replay-pruning functions. The Go readiness check must return nil for the actual processing connection and fail if any required signature or ACL differs.

### Behavioral verification

Use the PostgreSQL 17 verifier test harness against this staging database only after reviewing its fixture setup and ensuring it will not collide with existing staging objects. Verify valid signed context, transaction binding, rollback/retry, committed replay rejection, expiry, lease fencing, cross-tenant read/write denial, password-hash denial, role ceiling on user inserts, private key/outbox denial, and pooled-connection context isolation. The GitHub Actions test harness is the default disposable repeatable test; a staging run is additional evidence, not a substitute for it.

## Stop conditions

Stop and preserve logs (redacting secrets) if any installer fails, a role has unexpected membership, a table/function ACL differs from the expected inventory, any RLS policy is missing or unexpectedly permissive, or the application readiness check fails. Do not try to repair drift by granting broader privileges or by running as a superuser.

## Rollback

Rollback is permitted only in the disposable staging database and only after capturing the failure evidence. Prefer restoring the pre-install snapshot. Do not drop roles or schemas manually if any object is still owned by or depends on them. If snapshot restore is unavailable, have the database owner prepare and review a dependency-aware teardown plan before executing any destructive SQL.

After successful staging verification, keep the production worker on its existing path. A separate change request and explicit approval are required before provisioning production credentials, changing Render secrets, enabling the signed-context guard, deploying, or merging PR #363.
