# Bulk import worker deployment

## Process separation

The HTTP API no longer starts the bulk-import polling goroutine. The recommended no-cost staging runner is the scheduled GitHub Actions workflow at `.github/workflows/bulk-import-worker-staging.yml`. It starts the standalone `cmd/bulk-import-worker` every five minutes, polls for up to four minutes, and exits. The repository is public, so standard GitHub-hosted Ubuntu runners are free and unlimited. This is periodic processing, not an always-on worker; schedule delays are possible.

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

Configure these GitHub Actions repository secrets for the scheduled worker only:

- `DB_DRIVER=postgres`
- `BULK_IMPORT_PROCESS_DATABASE_URL`: existing staging application processing DSN (secret; currently privileged)
- `BULK_IMPORT_CLAIM_DATABASE_URL`: DSN for the restricted `stonez_bulk_worker` login via the Supabase Session pooler (secret)
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_FROM_NAME`, `RESEND_API_KEY`, and `APP_LOGIN_URL` if credential emails are enabled

The workflow maps `BULK_IMPORT_PROCESS_DATABASE_URL` to the worker process's `DATABASE_URL`. Never add `BULK_IMPORT_CLAIM_DATABASE_URL` to the HTTP API's Render environment.

Do not configure `BULK_IMPORT_CLAIM_DATABASE_URL` on the HTTP API service.
Do not configure a migration URL or run application migrations from this worker.

For Supabase shared Session pooler connections, use the role-specific username
format `stonez_bulk_worker.<project-ref>` and the host/port copied from the
project's Connect dialog. Keep the password in Render's secret environment
settings; never commit it or put it in a SQL file.

## Staging rollout sequence

1. Create the restricted worker login by running
   `psql "$STAGING_ADMIN_DATABASE_URL" -X -f scripts/security/provision_bulk_import_worker_login.psql`
   from an interactive terminal. The script prompts for the password without
   echoing it. Do not paste the password into chat or commit it.
2. Copy the restricted role's Supabase Session pooler connection string using
   the role-specific username `stonez_bulk_worker.<project-ref>`. Store it as
   the GitHub Actions secret `BULK_IMPORT_CLAIM_DATABASE_URL`.
3. Store the staging application processing connection string as
   `BULK_IMPORT_PROCESS_DATABASE_URL`. This remains the existing processing
   credential and is not changed on the API service.
4. Add the optional SMTP/Resend secrets if imports send credentials.
5. Review this PR. The scheduled workflow only activates after it is merged to
   the repository's default branch. Then use Actions -> Bulk Import Worker
   (Staging) -> Run workflow for a manual smoke test; the five-minute schedule
   runs automatically thereafter.
6. Submit a disposable staging import and verify queued -> running ->
   completed (or completed_with_errors), with expected records and tenant
   ownership. Confirm API Render environment does not contain the claim URL.
7. Keep production untouched.

## Optional paid always-on worker

If processing must be continuous rather than scheduled, Render Background
Workers are available but Render does not offer a free compute plan for them.
The included `Dockerfile.worker` can be used for that deployment. Do not create
or deploy a paid worker without explicit approval.

**Important transitional limitation:** the processing connection still uses the
existing application database identity. This isolates the global claim
capability from the HTTP API but does not yet make job processing itself
least-privileged. Complete the wider tenant RLS rollout and a restricted
processing-role design before any restricted application-processing or
production cutover.
