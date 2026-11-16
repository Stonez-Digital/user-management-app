# Bulk import worker deployment

## Process separation

The HTTP API no longer starts the bulk-import polling goroutine. Run the standalone
`cmd/bulk-import-worker` process as a separate Render Background Worker using
`Dockerfile.worker`.

The worker uses two database connections:

- `BULK_IMPORT_CLAIM_DATABASE_URL`: restricted login granted membership in
  `stonez_api_runtime`. It is used only to execute
  `stonez_internal.claim_next_bulk_import_job(uuid)`.
- `DATABASE_URL`: existing application processing connection used for tenant
  transactions, reading/writing users and student records, and updating job
  progress after a claim.

The claim connection is checked at startup. It must authenticate as a login
that is not SUPERUSER, BYPASSRLS, CREATEDB, or CREATEROLE; it must have effective
membership in `stonez_api_runtime`, no membership in
`stonez_bulk_queue_owner`, and EXECUTE on the claim function. The API service
must not receive the claim URL.

**Important transitional limitation:** the processing connection still uses the
existing application database identity. This isolates the global claim
capability from the HTTP API but does not yet make job processing itself
least-privileged. Do not enable the global `REQUIRE_NOBYPASSRLS` cutover or
change the main API database credential based on this worker split alone.
Complete the wider tenant RLS rollout and a restricted processing-role design
before that cutover.

## Environment

Configure only on the dedicated worker service:

- `DB_DRIVER=postgres`
- `DATABASE_URL`: existing application processing DSN (secret)
- `BULK_IMPORT_CLAIM_DATABASE_URL`: DSN for the restricted `stonez_bulk_worker`
  login via the Supabase Session pooler (secret)
- SMTP / Resend variables if credential emails are enabled

Do not configure `BULK_IMPORT_CLAIM_DATABASE_URL` on the HTTP API service.
Do not configure a migration URL or run application migrations from this worker.

For Supabase shared Session pooler connections, use the role-specific username
format `stonez_bulk_worker.<project-ref>` and the host/port copied from the
project's Connect dialog. Keep the password in Render's secret environment
settings; never commit it or put it in a SQL file.

## Staging rollout sequence

1. Create a dedicated `stonez_bulk_worker` LOGIN role on the staging project,
   with no elevated attributes, and grant only membership in
   `stonez_api_runtime`.
2. Set its password directly into the dedicated Render worker's secret
   environment. Do not send it in chat or commit it.
3. Create the Render Background Worker from this branch with Dockerfile path
   `./Dockerfile.worker`, and configure the variables above.
4. Verify worker startup logs report successful restricted claim-role validation.
5. Submit a disposable staging import and verify it transitions queued ->
   running -> completed (or completed_with_errors), with the expected records
   and tenant ownership.
6. Confirm the HTTP API has no claim URL and does not start a worker.
7. Keep the existing staging API and production API database credentials
   unchanged until separate approval.

Do not run this worker against production until a separate production rollout
is reviewed and approved.
