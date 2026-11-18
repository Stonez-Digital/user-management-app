-- Operator-run preparation only. Not called by app startup or migrations.
-- The queue owner is NOBYPASSRLS and receives global queue visibility only
-- through a dedicated policy on bulk_import_jobs.
-- The API group role is NOBYPASSRLS; the later function installer grants only
-- tenant-policy-controlled SELECT/INSERT/UPDATE on the queue table.
-- Do not grant either role to the current postgres API connection.
BEGIN;
DO $preflight$
BEGIN
    IF to_regclass('public.bulk_import_jobs') IS NULL THEN
        RAISE EXCEPTION 'public.bulk_import_jobs must exist before queue-role provisioning';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM pg_attribute
         WHERE attrelid = 'public.bulk_import_jobs'::regclass
           AND attname = 'lease_id'
           AND attnum > 0
           AND NOT attisdropped
    ) THEN
        RAISE EXCEPTION 'public.bulk_import_jobs.lease_id is missing; apply application migration 48 before queue-role provisioning';
    END IF;
END
$preflight$;

DO $
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner') THEN
        CREATE ROLE stonez_bulk_queue_owner;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_api_runtime') THEN
        CREATE ROLE stonez_api_runtime;
    END IF;
END
$$;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname IN ('stonez_bulk_queue_owner', 'stonez_api_runtime') AND rolsuper) THEN
        RAISE EXCEPTION 'queue roles must be created as non-superusers';
    END IF;
END
$$;
-- New roles default to NOCREATEDB and NOCREATEROLE. A CREATEROLE-only
-- operator cannot change another role's CREATEDB attribute, so leave these
-- safe defaults untouched and assert them in the verification script.
ALTER ROLE stonez_bulk_queue_owner NOLOGIN NOINHERIT;
ALTER ROLE stonez_api_runtime NOLOGIN INHERIT;
-- PostgreSQL does not let a non-superuser ALTER ROLE change SUPERUSER,
-- even to NOSUPERUSER. New roles default to NOSUPERUSER; assert it instead.

GRANT USAGE ON SCHEMA public TO stonez_bulk_queue_owner;
REVOKE CREATE ON SCHEMA public FROM stonez_bulk_queue_owner;
REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
REVOKE UPDATE (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
REVOKE INSERT (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
REVOKE REFERENCES (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_queue_owner;
GRANT SELECT ON TABLE public.bulk_import_jobs TO stonez_bulk_queue_owner;
GRANT UPDATE (status, worker_id, lease_id, claimed_at, attempts, last_error)
    ON TABLE public.bulk_import_jobs TO stonez_bulk_queue_owner;
REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE SELECT (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE INSERT (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE UPDATE (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
    rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
REVOKE REFERENCES (
    id, school_id, initiated_by, kind, file_name, status, total, valid, created,
    updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, lease_id, claimed_at, attempts, last_error,
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
