-- Install the global queue claim function after provisioning the two roles.
-- This script is operator-run and must be applied before deploying the Go worker
-- version that calls claim_next_bulk_import_job. It is not an app-startup migration.
BEGIN;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'stonez_bulk_queue_owner'
          AND rolcanlogin = false AND rolsuper = false AND rolbypassrls = true
          AND rolcreatedb = false AND rolcreaterole = false AND rolinherit = false
    ) THEN
        RAISE EXCEPTION 'stonez_bulk_queue_owner is missing or has unsafe attributes';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'stonez_api_runtime'
          AND rolcanlogin = false AND rolsuper = false AND rolbypassrls = false
          AND rolcreatedb = false AND rolcreaterole = false AND rolinherit = true
    ) THEN
        RAISE EXCEPTION 'stonez_api_runtime is missing or has unsafe attributes';
    END IF;

    IF pg_has_role('stonez_api_runtime', 'stonez_bulk_queue_owner', 'MEMBER')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'SELECT')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'INSERT')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'UPDATE')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'REFERENCES') THEN
        RAISE EXCEPTION 'API runtime must not inherit queue-owner privileges or access bulk_import_jobs directly';
    END IF;

    IF NOT has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'SELECT')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'status', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'worker_id', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'claimed_at', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'attempts', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'last_error', 'UPDATE')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'UPDATE')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'INSERT')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'DELETE')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'TRUNCATE')
       OR has_schema_privilege('stonez_bulk_queue_owner', 'public', 'CREATE') THEN
        RAISE EXCEPTION 'queue owner privileges are missing or broader than intended';
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION public.claim_next_bulk_import_job(p_worker_id uuid)
RETURNS SETOF public.bulk_import_jobs
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE
    v_job public.bulk_import_jobs%ROWTYPE;
BEGIN
    IF p_worker_id IS NULL THEN
        RAISE EXCEPTION 'worker ID is required';
    END IF;

    SELECT *
      INTO v_job
      FROM public.bulk_import_jobs
     WHERE status = 'queued'
        OR (status = 'running'
            AND (claimed_at IS NULL OR claimed_at < clock_timestamp() - interval '5 minutes'))
     ORDER BY created_at ASC, id ASC
     LIMIT 1
     FOR UPDATE SKIP LOCKED;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    UPDATE public.bulk_import_jobs
       SET status = 'running',
           worker_id = p_worker_id,
           claimed_at = clock_timestamp(),
           attempts = attempts + 1,
           last_error = ''
     WHERE id = v_job.id
     RETURNING * INTO v_job;

    RETURN NEXT v_job;
END
$function$;

-- PostgreSQL requires the new function owner to have CREATE on its schema at
-- ownership-transfer time. Grant it only for this transaction, then revoke.
GRANT USAGE, CREATE ON SCHEMA public TO stonez_bulk_queue_owner;
ALTER FUNCTION public.claim_next_bulk_import_job(uuid) OWNER TO stonez_bulk_queue_owner;
REVOKE CREATE ON SCHEMA public FROM stonez_bulk_queue_owner;

REVOKE ALL PRIVILEGES ON FUNCTION public.claim_next_bulk_import_job(uuid) FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION public.claim_next_bulk_import_job(uuid) TO stonez_api_runtime;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON FUNCTION public.claim_next_bulk_import_job(uuid) FROM anon';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON FUNCTION public.claim_next_bulk_import_job(uuid) FROM authenticated';
    END IF;
END
$$;
COMMIT;
