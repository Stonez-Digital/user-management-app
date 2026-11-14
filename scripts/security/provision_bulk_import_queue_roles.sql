-- Operator-run preparation only. Not called by app startup or migrations.
-- The queue owner is NOBYPASSRLS and receives global queue visibility only
-- through a dedicated policy on bulk_import_jobs.
-- The API group role is NOBYPASSRLS; the later function installer grants only
-- tenant-policy-controlled SELECT/INSERT/UPDATE on the queue table.
-- Do not grant either role to the current postgres API connection.
BEGIN;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner') THEN
        CREATE ROLE stonez_bulk_queue_owner;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_api_runtime') THEN
        CREATE ROLE stonez_api_runtime;
    END IF;
END
$$;
ALTER ROLE stonez_bulk_queue_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
ALTER ROLE stonez_api_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA public TO stonez_bulk_queue_owner;
REVOKE CREATE ON SCHEMA public FROM stonez_bulk_queue_owner;
REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
REVOKE UPDATE (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
REVOKE INSERT (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
REVOKE REFERENCES (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
GRANT SELECT ON TABLE public.bulk_import_jobs TO stonez_bulk_queue_owner;
GRANT UPDATE (status, worker_id, claimed_at, attempts, last_error)
    ON TABLE public.bulk_import_jobs TO stonez_bulk_queue_owner;
REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE SELECT (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE INSERT (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE UPDATE (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE REFERENCES (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM anon';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM authenticated';
    END IF;
END
$$;
COMMIT;
