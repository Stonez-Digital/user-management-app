-- Operator-run preparation only. Not called by app startup or migrations.
-- The queue owner is BYPASSRLS but has privileges only on bulk_import_jobs.
-- The API group role is NOBYPASSRLS and receives no direct queue-table access.
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
ALTER ROLE stonez_bulk_queue_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT BYPASSRLS;
ALTER ROLE stonez_api_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA public TO stonez_bulk_queue_owner;
REVOKE CREATE ON SCHEMA public FROM stonez_bulk_queue_owner;
GRANT SELECT ON TABLE public.bulk_import_jobs TO stonez_bulk_queue_owner;
GRANT UPDATE (status, worker_id, claimed_at, attempts, last_error)
    ON TABLE public.bulk_import_jobs TO stonez_bulk_queue_owner;
REVOKE ALL PRIVILEGES ON TABLE public.bulk_import_jobs FROM stonez_api_runtime;
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
