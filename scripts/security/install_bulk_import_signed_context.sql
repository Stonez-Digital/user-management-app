-- Operator-run installation of the signed bulk-import context verifier.
-- This script creates database objects only in the explicitly selected database.
-- It does not provision keys, enable pgcrypto, grant tenant-table access to the
-- processing login, or authorize production deployment.
\set ON_ERROR_STOP on
BEGIN;

DO $$
DECLARE
    v_hmac_schema text;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
         WHERE rolname = 'stonez_bulk_context_owner'
           AND NOT rolcanlogin AND NOT rolsuper AND NOT rolbypassrls
           AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolinherit
    ) THEN
        RAISE EXCEPTION 'stonez_bulk_context_owner is missing or has unsafe attributes';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
         WHERE rolname = 'stonez_bulk_process_group'
           AND NOT rolcanlogin AND NOT rolsuper AND NOT rolbypassrls
           AND NOT rolcreatedb AND NOT rolcreaterole AND rolinherit
    ) THEN
        RAISE EXCEPTION 'stonez_bulk_process_group is missing or has unsafe attributes';
    END IF;
    -- Resolve hmac through pg_extension membership, not a hard-coded schema.
    SELECT namespace.nspname
      INTO v_hmac_schema
      FROM pg_extension AS extension_row
      JOIN pg_namespace AS namespace ON namespace.oid = extension_row.extnamespace
      JOIN pg_depend AS dependency
        ON dependency.refclassid = 'pg_extension'::regclass
       AND dependency.refobjid = extension_row.oid
       AND dependency.classid = 'pg_proc'::regclass
       AND dependency.deptype = 'e'
      JOIN pg_proc AS procedure ON procedure.oid = dependency.objid
     WHERE extension_row.extname = 'pgcrypto'
       AND procedure.proname = 'hmac'
       AND procedure.proargtypes = '17 17 25'::oidvector;
    IF v_hmac_schema IS NULL THEN
        RAISE EXCEPTION 'pgcrypto extension with hmac(bytea,bytea,text) is required';
    END IF;
    EXECUTE format('GRANT USAGE ON SCHEMA %I TO stonez_bulk_context_owner', v_hmac_schema);
    EXECUTE format('GRANT EXECUTE ON FUNCTION %I.hmac(bytea,bytea,text) TO stonez_bulk_context_owner', v_hmac_schema);
    -- This accepts a pgcrypto installation in either public or extensions.
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = 'public'
           AND table_name = 'bulk_import_jobs'
           AND column_name = 'lease_generation'
           AND data_type = 'bigint'
    ) THEN
        RAISE EXCEPTION 'bulk_import_jobs.lease_generation BIGINT is required before installing the verifier';
    END IF;
END
$$;

CREATE SCHEMA IF NOT EXISTS stonez_internal;
REVOKE ALL PRIVILEGES ON SCHEMA stonez_internal FROM PUBLIC;
GRANT USAGE ON SCHEMA stonez_internal TO stonez_bulk_context_owner, stonez_bulk_process_group;
GRANT USAGE ON SCHEMA public TO stonez_bulk_context_owner, stonez_bulk_process_group;

-- Vault remains the source of truth. These rows are a protected verifier copy;
-- they are populated only by a separately approved key-delivery procedure.
CREATE TABLE IF NOT EXISTS stonez_internal.bulk_context_keys (
    key_id text PRIMARY KEY CHECK (key_id ~ '^[A-Za-z0-9._-]{1,64}$'),
    secret bytea NOT NULL CHECK (octet_length(secret) >= 32),
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- One row is both the durable replay record and the transaction-bound context.
-- The unique backend/transaction pair allows at most one assertion per txn.
CREATE TABLE IF NOT EXISTS stonez_internal.bulk_context_sessions (
    assertion_id uuid PRIMARY KEY,
    backend_pid integer NOT NULL,
    transaction_id xid8 NOT NULL,
    key_id text NOT NULL,
    job_id uuid NOT NULL,
    school_id uuid NOT NULL,
    initiated_by uuid NOT NULL,
    worker_id uuid NOT NULL,
    lease_generation bigint NOT NULL CHECK (lease_generation > 0),
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (backend_pid, transaction_id)
);
CREATE INDEX IF NOT EXISTS bulk_context_sessions_expires_at_idx
    ON stonez_internal.bulk_context_sessions (expires_at);

REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_keys FROM PUBLIC;
REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_sessions FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_keys FROM anon';
        EXECUTE 'REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_sessions FROM anon';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_keys FROM authenticated';
        EXECUTE 'REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_sessions FROM authenticated';
    END IF;
END
$$;
DO $$
DECLARE
    v_role text;
BEGIN
    FOREACH v_role IN ARRAY ARRAY[
        'anon', 'authenticated', 'service_role', 'stonez_api_runtime',
        'stonez_bulk_worker', 'stonez_bulk_process_login', 'stonez_bulk_process_group'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = v_role) THEN
            EXECUTE format('REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_keys FROM %I', v_role);
            EXECUTE format('REVOKE ALL PRIVILEGES ON TABLE stonez_internal.bulk_context_sessions FROM %I', v_role);
        END IF;
    END LOOP;
END
$$;

GRANT SELECT ON TABLE stonez_internal.bulk_context_keys TO stonez_bulk_context_owner;
GRANT SELECT, INSERT ON TABLE stonez_internal.bulk_context_sessions TO stonez_bulk_context_owner;

-- The verifier owner is not the queue owner. It gets only the queue visibility
-- and single-column UPDATE permission needed for SELECT ... FOR UPDATE.
GRANT SELECT ON TABLE public.bulk_import_jobs TO stonez_bulk_context_owner;
GRANT UPDATE (lease_generation) ON TABLE public.bulk_import_jobs TO stonez_bulk_context_owner;
ALTER TABLE public.bulk_import_jobs ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS bulk_import_jobs_context_owner_select ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_context_owner_select
    ON public.bulk_import_jobs FOR SELECT TO stonez_bulk_context_owner
    USING (true);
DROP POLICY IF EXISTS bulk_import_jobs_context_owner_update ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_context_owner_update
    ON public.bulk_import_jobs FOR UPDATE TO stonez_bulk_context_owner
    USING (true) WITH CHECK (true);

GRANT USAGE, CREATE ON SCHEMA stonez_internal TO stonez_bulk_context_owner;
GRANT stonez_bulk_context_owner TO CURRENT_USER WITH INHERIT FALSE, SET TRUE;
SET LOCAL ROLE stonez_bulk_context_owner;

-- Create a private, schema-qualified adapter to the hmac function that belongs
-- to the installed pgcrypto extension. The verifier never adds that schema to
-- search_path and the processing group cannot execute this adapter directly.
DO $pgcrypto$
DECLARE
    v_hmac_schema text;
    v_definition text;
BEGIN
    SELECT namespace.nspname
      INTO v_hmac_schema
      FROM pg_extension AS extension_row
      JOIN pg_namespace AS namespace ON namespace.oid = extension_row.extnamespace
      JOIN pg_depend AS dependency
        ON dependency.refclassid = 'pg_extension'::regclass
       AND dependency.refobjid = extension_row.oid
       AND dependency.classid = 'pg_proc'::regclass
       AND dependency.deptype = 'e'
      JOIN pg_proc AS procedure ON procedure.oid = dependency.objid
     WHERE extension_row.extname = 'pgcrypto'
       AND procedure.proname = 'hmac'
       AND procedure.proargtypes = '17 17 25'::oidvector;
    IF v_hmac_schema IS NULL THEN
        RAISE EXCEPTION 'pgcrypto hmac function disappeared during signed-context installation';
    END IF;

    v_definition := format(
        'CREATE OR REPLACE FUNCTION stonez_internal.bulk_import_hmac(bytea, bytea, text) RETURNS bytea LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE SET search_path = pg_catalog AS %L',
        format('SELECT %I.hmac($1, $2, $3)', v_hmac_schema)
    );
    EXECUTE v_definition;
END
$pgcrypto$;
REVOKE ALL PRIVILEGES ON FUNCTION stonez_internal.bulk_import_hmac(bytea,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.bulk_import_hmac(bytea,bytea,text) TO stonez_bulk_context_owner;

CREATE OR REPLACE FUNCTION stonez_internal.verify_bulk_import_context(
    p_key_id text,
    p_assertion_id uuid,
    p_job_id uuid,
    p_school_id uuid,
    p_initiated_by uuid,
    p_worker_id uuid,
    p_backend_pid integer,
    p_transaction_id text,
    p_lease_generation bigint,
    p_issued_at bigint,
    p_expires_at bigint,
    p_signature text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
SET row_security = on
AS $function$
DECLARE
    v_secret bytea;
    v_payload text;
    v_expected bytea;
    v_provided bytea;
    v_diff integer := 0;
    v_index integer;
    v_now_epoch bigint;
    v_school_id uuid;
    v_initiated_by uuid;
    v_worker_id uuid;
    v_lease_generation bigint;
    v_status text;
    v_claimed_at timestamptz;
    v_rows integer;
BEGIN
    IF p_key_id IS NULL OR p_key_id !~ '^[A-Za-z0-9._-]{1,64}$'
       OR p_assertion_id IS NULL OR p_job_id IS NULL OR p_school_id IS NULL
       OR p_initiated_by IS NULL OR p_worker_id IS NULL
       OR p_backend_pid IS NULL OR p_backend_pid <= 0
       OR p_transaction_id IS NULL OR p_transaction_id !~ '^[1-9][0-9]{0,19}$'
       OR p_lease_generation IS NULL OR p_lease_generation <= 0
       OR p_issued_at IS NULL OR p_issued_at <= 0
       OR p_expires_at IS NULL OR p_expires_at <= p_issued_at
       OR p_expires_at - p_issued_at > 120
       OR p_signature IS NULL OR p_signature !~ '^[A-Za-z0-9_-]{43}$' THEN
        RAISE EXCEPTION 'invalid bulk import context assertion' USING ERRCODE = '28000';
    END IF;

    IF p_backend_pid <> pg_backend_pid() OR p_transaction_id <> pg_current_xact_id()::text THEN
        RAISE EXCEPTION 'bulk import context assertion belongs to another PostgreSQL transaction' USING ERRCODE = '28000';
    END IF;

    v_now_epoch := floor(extract(epoch FROM clock_timestamp()))::bigint;
    IF p_issued_at > v_now_epoch + 30 OR p_expires_at <= v_now_epoch THEN
        RAISE EXCEPTION 'invalid bulk import context assertion lifetime' USING ERRCODE = '28000';
    END IF;

    SELECT key_row.secret
      INTO v_secret
      FROM stonez_internal.bulk_context_keys AS key_row
     WHERE key_row.key_id = p_key_id
       AND key_row.enabled;
    IF NOT FOUND OR octet_length(v_secret) < 32 THEN
        RAISE EXCEPTION 'invalid bulk import context assertion' USING ERRCODE = '28000';
    END IF;

    v_payload := concat_ws(E'\n',
        'stonez-bulk-import-context-v1',
        p_key_id,
        p_assertion_id::text,
        p_job_id::text,
        p_school_id::text,
        p_initiated_by::text,
        p_worker_id::text,
        p_backend_pid::text,
        p_transaction_id,
        p_lease_generation::text,
        p_issued_at::text,
        p_expires_at::text
    );
    v_expected := stonez_internal.bulk_import_hmac(convert_to(v_payload, 'UTF8'), v_secret, 'sha256');
    v_provided := decode(
        translate(p_signature, '-_', '+/') ||
        repeat('=', (4 - length(p_signature) % 4) % 4),
        'base64'
    );
    IF octet_length(v_expected) <> 32 OR octet_length(v_provided) <> 32 THEN
        RAISE EXCEPTION 'invalid bulk import context assertion' USING ERRCODE = '28000';
    END IF;

    -- Fixed-work byte comparison: do not return early on the first mismatch.
    FOR v_index IN 0..31 LOOP
        v_diff := v_diff | (get_byte(v_expected, v_index) # get_byte(v_provided, v_index));
    END LOOP;
    IF v_diff <> 0 THEN
        RAISE EXCEPTION 'invalid bulk import context assertion' USING ERRCODE = '28000';
    END IF;

    SELECT job.school_id, job.initiated_by, job.worker_id,
           job.lease_generation, job.status, job.claimed_at
      INTO v_school_id, v_initiated_by, v_worker_id,
           v_lease_generation, v_status, v_claimed_at
      FROM public.bulk_import_jobs AS job
     WHERE job.id = p_job_id
     FOR UPDATE;
    IF NOT FOUND
       OR v_school_id IS DISTINCT FROM p_school_id
       OR v_initiated_by IS DISTINCT FROM p_initiated_by
       OR v_worker_id IS DISTINCT FROM p_worker_id
       OR v_lease_generation IS DISTINCT FROM p_lease_generation
       OR v_status IS DISTINCT FROM 'running'
       OR v_claimed_at IS NULL
       OR v_claimed_at < clock_timestamp() - interval '5 minutes' THEN
        RAISE EXCEPTION 'bulk import worker lease is no longer valid' USING ERRCODE = '28000';
    END IF;

    INSERT INTO stonez_internal.bulk_context_sessions (
        assertion_id, backend_pid, transaction_id, key_id, job_id,
        school_id, initiated_by, worker_id, lease_generation,
        issued_at, expires_at
    ) VALUES (
        p_assertion_id, pg_backend_pid(), pg_current_xact_id(), p_key_id, p_job_id,
        p_school_id, p_initiated_by, p_worker_id, p_lease_generation,
        to_timestamp(p_issued_at), to_timestamp(p_expires_at)
    );
    GET DIAGNOSTICS v_rows = ROW_COUNT;
    IF v_rows <> 1 THEN
        RAISE EXCEPTION 'bulk import context assertion was already consumed' USING ERRCODE = '28000';
    END IF;

    PERFORM set_config('stonez.bulk_context_id', p_assertion_id::text, true);
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.current_bulk_import_context()
RETURNS TABLE (
    job_id uuid,
    school_id uuid,
    initiated_by uuid,
    worker_id uuid,
    lease_generation bigint,
    expires_at timestamptz
)
LANGUAGE sql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog
SET row_security = on
AS $function$
    SELECT session_row.job_id, session_row.school_id, session_row.initiated_by,
           session_row.worker_id, session_row.lease_generation, session_row.expires_at
      FROM stonez_internal.bulk_context_sessions AS session_row
     WHERE session_row.assertion_id =
           nullif(current_setting('stonez.bulk_context_id', true), '')::uuid
       AND session_row.backend_pid = pg_backend_pid()
       AND session_row.transaction_id = pg_current_xact_id()
       AND session_row.expires_at > clock_timestamp()
       AND EXISTS (
            SELECT 1
              FROM public.bulk_import_jobs AS job
             WHERE job.id = session_row.job_id
               AND job.school_id = session_row.school_id
               AND job.initiated_by = session_row.initiated_by
               AND job.lease_generation = session_row.lease_generation
               AND (
                    (job.status = 'running'
                     AND job.worker_id = session_row.worker_id
                     AND job.claimed_at >= clock_timestamp() - interval '5 minutes')
                    OR
                    (job.status IN ('completed', 'completed_with_errors', 'failed')
                     AND job.worker_id IS NULL
                     AND job.claimed_at IS NULL)
               )
       )
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.bulk_import_context_key_available(p_key_id text)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
SET row_security = on
AS $function$
    SELECT EXISTS (
        SELECT 1
          FROM stonez_internal.bulk_context_keys AS key_row
         WHERE key_row.key_id = p_key_id
           AND key_row.enabled
           AND octet_length(key_row.secret) >= 32
    )
$function$;

REVOKE ALL PRIVILEGES ON FUNCTION stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text) FROM PUBLIC;
REVOKE ALL PRIVILEGES ON FUNCTION stonez_internal.current_bulk_import_context() FROM PUBLIC;
REVOKE ALL PRIVILEGES ON FUNCTION stonez_internal.bulk_import_context_key_available(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text) TO stonez_bulk_process_group;
GRANT EXECUTE ON FUNCTION stonez_internal.current_bulk_import_context() TO stonez_bulk_process_group;
GRANT EXECUTE ON FUNCTION stonez_internal.bulk_import_context_key_available(text) TO stonez_bulk_process_group;

-- Remove any pre-existing direct EXECUTE grants. Only the dedicated processing
-- group may call these functions; the NOLOGIN owner retains ownership rights.
DO $$
DECLARE
    v_grantee text;
    v_function regprocedure;
BEGIN
    FOREACH v_function IN ARRAY ARRAY[
        'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)'::regprocedure,
        'stonez_internal.current_bulk_import_context()'::regprocedure,
        'stonez_internal.bulk_import_context_key_available(text)'::regprocedure
    ] LOOP
        FOR v_grantee IN
            SELECT DISTINCT role_row.rolname
              FROM pg_proc AS procedure
              CROSS JOIN LATERAL aclexplode(COALESCE(procedure.proacl, acldefault('f', procedure.proowner))) AS privilege
              JOIN pg_roles AS role_row ON role_row.oid = privilege.grantee
             WHERE procedure.oid = v_function
               AND privilege.privilege_type = 'EXECUTE'
               AND role_row.rolname NOT IN ('stonez_bulk_context_owner', 'stonez_bulk_process_group')
        LOOP
            EXECUTE format('REVOKE ALL PRIVILEGES ON FUNCTION %s FROM %I', v_function, v_grantee);
        END LOOP;
    END LOOP;
END
$$;

-- The claim function returns the job payload to the worker. The processing
-- connection needs only the key columns used by its guarded UPDATE predicate;
-- it must not be able to SELECT rows_json or other payload columns.
RESET ROLE;

REVOKE SELECT ON TABLE public.bulk_import_jobs FROM stonez_bulk_process_group;
REVOKE SELECT (
    id, school_id, initiated_by, kind, file_name, status, total, valid,
    created, updated, skipped, failed, send_credentials, credentials_sent,
    credential_emails_failed, worker_id, claimed_at, attempts, lease_generation,
    last_error, rows_json, errors_json, created_at, updated_at
) ON TABLE public.bulk_import_jobs FROM stonez_bulk_process_group;
GRANT SELECT (id, school_id, initiated_by, worker_id)
    ON TABLE public.bulk_import_jobs TO stonez_bulk_process_group;
GRANT UPDATE (
    status, created, skipped, failed, credentials_sent,
    credential_emails_failed, errors_json, claimed_at, worker_id,
    last_error, updated_at
) ON TABLE public.bulk_import_jobs TO stonez_bulk_process_group;

DROP POLICY IF EXISTS bulk_import_jobs_processing_select ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_processing_select
    ON public.bulk_import_jobs FOR SELECT TO stonez_bulk_process_group
    USING (
        EXISTS (
            SELECT 1 FROM stonez_internal.current_bulk_import_context() AS ctx
             WHERE ctx.job_id = bulk_import_jobs.id
               AND ctx.school_id = bulk_import_jobs.school_id
               AND ctx.initiated_by = bulk_import_jobs.initiated_by
        )
    );

-- PostgreSQL permissive policies are OR-combined. Keep an independent
-- restrictive guard so a pre-existing PUBLIC or role-wide policy cannot
-- widen the processing login beyond its verified job context.
DROP POLICY IF EXISTS bulk_import_jobs_processing_select_guard ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_processing_select_guard
    ON public.bulk_import_jobs AS RESTRICTIVE FOR SELECT TO stonez_bulk_process_group
    USING (
        EXISTS (
            SELECT 1 FROM stonez_internal.current_bulk_import_context() AS ctx
             WHERE ctx.job_id = bulk_import_jobs.id
               AND ctx.school_id = bulk_import_jobs.school_id
               AND ctx.initiated_by = bulk_import_jobs.initiated_by
        )
    );

DROP POLICY IF EXISTS bulk_import_jobs_processing_update ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_processing_update
    ON public.bulk_import_jobs FOR UPDATE TO stonez_bulk_process_group
    USING (
        EXISTS (
            SELECT 1 FROM stonez_internal.current_bulk_import_context() AS ctx
             WHERE ctx.job_id = bulk_import_jobs.id
               AND ctx.school_id = bulk_import_jobs.school_id
               AND ctx.initiated_by = bulk_import_jobs.initiated_by
        )
    )
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM stonez_internal.current_bulk_import_context() AS ctx
             WHERE ctx.job_id = bulk_import_jobs.id
               AND ctx.school_id = bulk_import_jobs.school_id
               AND ctx.initiated_by = bulk_import_jobs.initiated_by
        )
    );

DROP POLICY IF EXISTS bulk_import_jobs_processing_update_guard ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_processing_update_guard
    ON public.bulk_import_jobs AS RESTRICTIVE FOR UPDATE TO stonez_bulk_process_group
    USING (
        EXISTS (
            SELECT 1 FROM stonez_internal.current_bulk_import_context() AS ctx
             WHERE ctx.job_id = bulk_import_jobs.id
               AND ctx.school_id = bulk_import_jobs.school_id
               AND ctx.initiated_by = bulk_import_jobs.initiated_by
        )
    )
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM stonez_internal.current_bulk_import_context() AS ctx
             WHERE ctx.job_id = bulk_import_jobs.id
               AND ctx.school_id = bulk_import_jobs.school_id
               AND ctx.initiated_by = bulk_import_jobs.initiated_by
        )
    );

RESET ROLE;
REVOKE stonez_bulk_context_owner FROM CURRENT_USER;
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_bulk_context_owner;

DO $$
DECLARE
    v_owner text;
    v_config text[];
BEGIN
    SELECT owner_role.rolname, procedure.proconfig
      INTO v_owner, v_config
      FROM pg_proc AS procedure
      JOIN pg_roles AS owner_role ON owner_role.oid = procedure.proowner
     WHERE procedure.oid = 'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)'::regprocedure;
    IF v_owner IS DISTINCT FROM 'stonez_bulk_context_owner'
       OR NOT ('search_path=pg_catalog' = ANY(COALESCE(v_config, ARRAY[]::text[]))) THEN
        RAISE EXCEPTION 'signed-context verifier owner or search_path is unsafe';
    END IF;
    IF has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_keys', 'SELECT')
       OR has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_sessions', 'SELECT')
       OR has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_sessions', 'INSERT') THEN
        RAISE EXCEPTION 'processing group has direct access to protected verifier tables';
    END IF;
END
$$;

COMMIT;
