# Bulk-import claim-only PostgreSQL capability

Status: implementation in review; not approved for production or staging installation.

## Capability boundary

The worker uses two separate PostgreSQL logins:

- Processing login: member of `stonez_bulk_process_group`; performs tenant-scoped import work under signed transaction context.
- Claim login: member only of `stonez_bulk_queue_claim`; can execute `stonez_internal.claim_next_bulk_import_job(uuid)` and has no direct queue-table or tenant-table privileges.

The claim login must be a direct PostgreSQL login with `NOSUPERUSER`, `NOBYPASSRLS`, `NOCREATEDB`, and `NOCREATEROLE`. It must inherit the claim group but must not be able to `SET ROLE` to that group, the queue owner, or `stonez_api_runtime`. It must not be a member of either runtime or queue-owner roles.

The SECURITY DEFINER function is owned by the NOLOGIN `stonez_bulk_queue_owner`, which has only the queue SELECT privilege and claim-column UPDATE privileges, with a table-specific RLS policy. Function execution is granted only to `stonez_bulk_queue_claim`; PUBLIC and `stonez_api_runtime` are denied.

## Operator sequence (isolated environment only)

1. Create the disposable database and non-superuser CREATEROLE installer.
2. Run `scripts/security/provision_bulk_import_queue_roles.sql`.
3. Run `scripts/security/verify_bulk_import_queue_roles.sql` before installing the function.
4. Run `scripts/security/install_bulk_import_claim_function.sql`.
5. Run `scripts/security/provision_bulk_import_worker_login.psql` interactively to create the dedicated login.
6. Set `BULK_IMPORT_CLAIM_DATABASE_URL` only on the worker service, independently of its processing credential.
7. Run the PostgreSQL 17 acceptance workflow before any staging installation.

Do not run these operator scripts against production as part of this PR. Provisioning and installation require a separately reviewed staging target and explicit approval.

## Automated acceptance coverage

`.github/workflows/bulk-import-claim-role-postgres.yml` runs the real scripts against a disposable PostgreSQL 17 service. It verifies:

- The worker login inherits the claim capability and no other group.
- The claim function returns and leases a seeded queue job.
- The login cannot directly SELECT, INSERT, UPDATE, or DELETE queue rows.
- The login cannot read or mutate a tenant-table probe.
- The login cannot `SET ROLE` to the queue owner or API runtime.
- `stonez_api_runtime` does not have EXECUTE on the global claim function.
- An unrelated login and the privileged installer cannot execute the function.

These tests prove the SQL scripts and role boundary in disposable PostgreSQL only. They do not prove a managed staging database has been provisioned or configured.
