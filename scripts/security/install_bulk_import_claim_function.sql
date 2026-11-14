-- Install the global queue claim function after provisioning the two roles.
-- This script is operator-run and must be applied before deploying the Go worker
-- version that calls claim_next_bulk_import_job. It is not an app-startup migration.
BEGIN;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'stonez_bulk_queue_owner'
          AND rolcanlogin = false AND rolsuper = false AND rolbypassrls = false
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
       OR EXISTS (SELECT 1 FROM pg_auth_members WHERE member = (SELECT oid FROM pg_roles WHERE rolname = 'stonez_api_runtime') OR roleid = (SELECT oid FROM pg_roles WHERE rolname = 'stonez_api_runtime'))
       OR EXISTS (SELECT 1 FROM pg_auth_members WHERE member = (SELECT oid FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner') OR roleid = (SELECT oid FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner'))
       OR EXISTS (
            SELECT 1 FROM pg_roles AS privileged
             WHERE (privileged.rolsuper OR privileged.rolbypassrls)
               AND privileged.rolname NOT IN ('stonez_api_runtime', 'stonez_bulk_queue_owner')
               AND pg_has_role('stonez_api_runtime', privileged.oid, 'MEMBER')
       )
       OR EXISTS (
            SELECT 1 FROM pg_roles AS privileged
             WHERE (privileged.rolsuper OR privileged.rolbypassrls)
               AND privileged.rolname <> 'stonez_bulk_queue_owner'
               AND pg_has_role('stonez_bulk_queue_owner', privileged.oid, 'MEMBER')
       )
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'REFERENCES')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'DELETE')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'TRUNCATE')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'TRIGGER')
       OR EXISTS (
            SELECT 1 FROM pg_class AS relation
             WHERE relation.oid = 'public.bulk_import_jobs'::regclass
               AND relation.relowner IN (
                   (SELECT oid FROM pg_roles WHERE rolname = 'stonez_api_runtime'),
                   (SELECT oid FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner')
               )
       ) THEN
        RAISE EXCEPTION 'API runtime must not inherit queue-owner privileges, own the queue table, or have non-tenant write privileges';
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
       OR has_any_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'INSERT')
       OR has_any_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'REFERENCES')
       OR has_schema_privilege('stonez_bulk_queue_owner', 'public', 'CREATE')
       OR has_schema_privilege('stonez_api_runtime', 'public', 'CREATE') THEN
        RAISE EXCEPTION 'queue owner privileges are missing or broader than intended';
    END IF;
END
$$;

-- Keep the privileged function outside Supabase's exposed public schema.
CREATE SCHEMA IF NOT EXISTS stonez_internal AUTHORIZATION CURRENT_USER;
REVOKE ALL PRIVILEGES ON SCHEMA stonez_internal FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON SCHEMA stonez_internal FROM anon';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON SCHEMA stonez_internal FROM authenticated';
    END IF;
END
$$;
GRANT USAGE ON SCHEMA stonez_internal TO stonez_api_runtime;

-- Normal school-facing API operations keep tenant-scoped table access. The
-- global worker claim remains isolated behind the SECURITY DEFINER function.
-- The function owner is NOBYPASSRLS; this table-specific policy grants it
-- global queue visibility without granting cluster-wide RLS bypass.
ALTER TABLE public.bulk_import_jobs ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS bulk_import_jobs_queue_owner_all ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_queue_owner_all
    ON public.bulk_import_jobs
    FOR ALL TO stonez_bulk_queue_owner
    USING (true)
    WITH CHECK (true);
DROP POLICY IF EXISTS bulk_import_jobs_tenant_select ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_tenant_select
    ON public.bulk_import_jobs
    FOR SELECT TO stonez_api_runtime
    USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);

DROP POLICY IF EXISTS bulk_import_jobs_tenant_insert ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_tenant_insert
    ON public.bulk_import_jobs
    FOR INSERT TO stonez_api_runtime
    WITH CHECK (
        school_id = nullif(current_setting('app.current_school_id', true), '')::uuid
        AND initiated_by = nullif(current_setting('app.current_user_id', true), '')::uuid
    );

DROP POLICY IF EXISTS bulk_import_jobs_tenant_update ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_tenant_update
    ON public.bulk_import_jobs
    FOR UPDATE TO stonez_api_runtime
    USING (
        school_id = nullif(current_setting('app.current_school_id', true), '')::uuid
        AND initiated_by = nullif(current_setting('app.current_user_id', true), '')::uuid
    )
    WITH CHECK (
        school_id = nullif(current_setting('app.current_school_id', true), '')::uuid
        AND initiated_by = nullif(current_setting('app.current_user_id', true), '')::uuid
    );

GRANT SELECT, INSERT, UPDATE ON TABLE public.bulk_import_jobs TO stonez_api_runtime;

-- Create/replace the function while SET ROLE is the dedicated owner. This
-- avoids leaving the migration/operator role as function owner.
GRANT USAGE ON SCHEMA public TO stonez_api_runtime;
GRANT USAGE, CREATE ON SCHEMA stonez_internal TO stonez_bulk_queue_owner;
GRANT stonez_bulk_queue_owner TO CURRENT_USER;
SET LOCAL ROLE stonez_bulk_queue_owner;

CREATE OR REPLACE FUNCTION stonez_internal.claim_next_bulk_import_job(p_worker_id uuid)
RETURNS SETOF public.bulk_import_jobs
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE
    v_job public.bulk_import_jobs%ROWTYPE;
BEGIN
    IF NOT pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER') THEN
        RAISE EXCEPTION 'caller is not an authorized API runtime login';
    END IF;

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

REVOKE ALL PRIVILEGES ON FUNCTION stonez_internal.claim_next_bulk_import_job(uuid) FROM PUBLIC;
DO $$
DECLARE
    grantee_name text;
BEGIN
    FOR grantee_name IN
        SELECT grantee_role.rolname
          FROM pg_proc AS procedure
          CROSS JOIN LATERAL aclexplode(COALESCE(procedure.proacl, acldefault('f', procedure.proowner))) AS function_privilege
          JOIN pg_roles AS grantee_role ON grantee_role.oid = function_privilege.grantee
         WHERE procedure.oid = 'stonez_internal.claim_next_bulk_import_job(uuid)'::regprocedure
           AND grantee_role.rolname NOT IN ('stonez_bulk_queue_owner', 'stonez_api_runtime')
    LOOP
        EXECUTE format(
            'REVOKE ALL PRIVILEGES ON FUNCTION stonez_internal.claim_next_bulk_import_job(uuid) FROM %I',
            grantee_name
        );
    END LOOP;
END
$$;
GRANT EXECUTE ON FUNCTION stonez_internal.claim_next_bulk_import_job(uuid) TO stonez_api_runtime;

RESET ROLE;
REVOKE stonez_bulk_queue_owner FROM CURRENT_USER;
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_bulk_queue_owner;

DO $$
DECLARE
    function_owner text;
    function_is_definer boolean;
    function_config text[];
    function_definition text;
BEGIN
    SELECT owner_role.rolname, procedure.prosecdef, procedure.proconfig, pg_get_functiondef(procedure.oid)
      INTO function_owner, function_is_definer, function_config, function_definition
      FROM pg_proc AS procedure
      JOIN pg_namespace AS namespace ON namespace.oid = procedure.pronamespace
      JOIN pg_roles AS owner_role ON owner_role.oid = procedure.proowner
     WHERE procedure.oid = 'stonez_internal.claim_next_bulk_import_job(uuid)'::regprocedure;

    IF function_owner IS DISTINCT FROM 'stonez_bulk_queue_owner'
       OR function_is_definer IS DISTINCT FROM true
       OR NOT ('search_path=pg_catalog' = ANY(COALESCE(function_config, ARRAY[]::text[])))
       OR position('pg_has_role(session_user' IN function_definition) = 0 THEN
        RAISE EXCEPTION 'claim function owner, SECURITY DEFINER mode, or fixed search_path is unsafe';
    END IF;

    IF EXISTS (
            SELECT 1
              FROM pg_proc AS procedure
              CROSS JOIN LATERAL aclexplode(COALESCE(procedure.proacl, acldefault('f', procedure.proowner))) AS function_privilege
             WHERE procedure.oid = 'stonez_internal.claim_next_bulk_import_job(uuid)'::regprocedure
               AND function_privilege.privilege_type = 'EXECUTE'
               AND (function_privilege.grantee = 0
                    OR function_privilege.grantee NOT IN (
                        procedure.proowner,
                        (SELECT oid FROM pg_roles WHERE rolname = 'stonez_api_runtime')
                    ))
       )
       OR EXISTS (
            SELECT 1
              FROM pg_namespace AS private_schema
              CROSS JOIN LATERAL aclexplode(COALESCE(private_schema.nspacl, acldefault('n', private_schema.nspowner))) AS schema_privilege
             WHERE private_schema.nspname = 'stonez_internal'
               AND (schema_privilege.grantee = 0
                    OR schema_privilege.grantee NOT IN (
                        private_schema.nspowner,
                        (SELECT oid FROM pg_roles WHERE rolname = 'stonez_api_runtime'),
                        (SELECT oid FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner')
                    ))
       )
       OR NOT has_schema_privilege('stonez_api_runtime', 'stonez_internal', 'USAGE')
       OR has_schema_privilege('stonez_api_runtime', 'stonez_internal', 'CREATE')
       OR has_schema_privilege('stonez_api_runtime', 'public', 'CREATE')
       OR has_schema_privilege('stonez_bulk_queue_owner', 'stonez_internal', 'CREATE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.claim_next_bulk_import_job(uuid)', 'EXECUTE')
       OR NOT EXISTS (
            SELECT 1 FROM pg_class
             WHERE oid = 'public.bulk_import_jobs'::regclass
               AND relrowsecurity
       )
       OR NOT has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'SELECT')
       OR NOT has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'INSERT')
       OR NOT has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'UPDATE')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'DELETE')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'TRUNCATE')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'TRIGGER')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'REFERENCES')
       OR has_any_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'INSERT')
       OR has_any_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'REFERENCES')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'REFERENCES') THEN
        RAISE EXCEPTION 'API runtime table/function grants do not match the intended tenant-only boundary';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'bulk_import_jobs' AND policyname = 'bulk_import_jobs_queue_owner_all')
       OR NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'bulk_import_jobs' AND policyname = 'bulk_import_jobs_tenant_select')
       OR NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'bulk_import_jobs' AND policyname = 'bulk_import_jobs_tenant_insert')
       OR NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'bulk_import_jobs' AND policyname = 'bulk_import_jobs_tenant_update') THEN
        RAISE EXCEPTION 'bulk import tenant RLS policies were not installed';
    END IF;
END
$$;
COMMIT;
