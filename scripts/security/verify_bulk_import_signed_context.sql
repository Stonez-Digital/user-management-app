-- Read-only verification of the signed bulk-import context boundary.
-- Does not select or print key material and does not modify database state.
DO $$
DECLARE
    v_owner oid;
    v_process oid;
    v_verifier_owner text;
    v_context_owner text;
    v_key_check_owner text;
    v_config text[];
BEGIN
    SELECT oid INTO v_owner FROM pg_roles WHERE rolname = 'stonez_bulk_context_owner';
    SELECT oid INTO v_process FROM pg_roles WHERE rolname = 'stonez_bulk_process_group';
    IF v_owner IS NULL OR v_process IS NULL THEN
        RAISE EXCEPTION 'signed-context owner or processing group role is missing';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles
         WHERE rolname = 'stonez_bulk_context_owner'
           AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'signed-context owner role attributes are unsafe';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles
         WHERE rolname = 'stonez_bulk_process_group'
           AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR NOT rolinherit)
    ) THEN
        RAISE EXCEPTION 'processing group role attributes are unsafe';
    END IF;
    IF to_regprocedure('extensions.hmac(bytea,bytea,text)') IS NULL
       OR to_regprocedure('stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)') IS NULL
       OR to_regprocedure('stonez_internal.current_bulk_import_context()') IS NULL
       OR to_regprocedure('stonez_internal.bulk_import_context_key_available(text)') IS NULL THEN
        RAISE EXCEPTION 'pgcrypto or signed-context functions are missing';
    END IF;

    SELECT owner_role.rolname, procedure.proconfig
      INTO v_verifier_owner, v_config
      FROM pg_proc procedure
      JOIN pg_roles owner_role ON owner_role.oid = procedure.proowner
     WHERE procedure.oid = 'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)'::regprocedure;
    IF v_verifier_owner IS DISTINCT FROM 'stonez_bulk_context_owner'
       OR NOT ('search_path=pg_catalog' = ANY(COALESCE(v_config, ARRAY[]::text[]))) THEN
        RAISE EXCEPTION 'verifier owner or fixed search_path is unsafe';
    END IF;

    SELECT owner_role.rolname INTO v_context_owner
      FROM pg_proc procedure
      JOIN pg_roles owner_role ON owner_role.oid = procedure.proowner
     WHERE procedure.oid = 'stonez_internal.current_bulk_import_context()'::regprocedure;
    SELECT owner_role.rolname INTO v_key_check_owner
      FROM pg_proc procedure
      JOIN pg_roles owner_role ON owner_role.oid = procedure.proowner
     WHERE procedure.oid = 'stonez_internal.bulk_import_context_key_available(text)'::regprocedure;
    IF v_context_owner IS DISTINCT FROM 'stonez_bulk_context_owner'
       OR v_key_check_owner IS DISTINCT FROM 'stonez_bulk_context_owner' THEN
        RAISE EXCEPTION 'context helper or key availability function has an unexpected owner';
    END IF;

    IF NOT has_function_privilege('stonez_bulk_process_group', 'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_bulk_process_group', 'stonez_internal.current_bulk_import_context()', 'EXECUTE')
       OR NOT has_function_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_import_context_key_available(text)', 'EXECUTE') THEN
        RAISE EXCEPTION 'processing group is missing required signed-context function access';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_proc procedure
          CROSS JOIN LATERAL aclexplode(COALESCE(procedure.proacl, acldefault('f', procedure.proowner))) AS privilege
         WHERE procedure.oid IN (
             'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)'::regprocedure,
             'stonez_internal.current_bulk_import_context()'::regprocedure,
             'stonez_internal.bulk_import_context_key_available(text)'::regprocedure
         )
           AND privilege.grantee = 0
           AND privilege.privilege_type = 'EXECUTE'
    ) THEN
        RAISE EXCEPTION 'PUBLIC has signed-context function access';
    END IF;

    IF has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_keys', 'SELECT')
       OR has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_sessions', 'SELECT')
       OR has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_sessions', 'INSERT')
       OR has_table_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'UPDATE') THEN
        RAISE EXCEPTION 'processing group has direct verifier-table access or broad queue UPDATE';
    END IF;
    IF has_table_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'SELECT')
       OR NOT has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'id', 'SELECT')
       OR NOT has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'school_id', 'SELECT')
       OR NOT has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'initiated_by', 'SELECT')
       OR NOT has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'worker_id', 'SELECT')
       OR has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'rows_json', 'SELECT')
       OR has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'errors_json', 'SELECT')
       OR has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'kind', 'SELECT')
       OR has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'file_name', 'SELECT')
       OR has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'lease_generation', 'SELECT')
       OR NOT has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'status', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'errors_json', 'UPDATE')
       OR has_column_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'lease_generation', 'UPDATE')
       OR has_table_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'INSERT')
       OR has_table_privilege('stonez_bulk_process_group', 'public.bulk_import_jobs', 'DELETE') THEN
        RAISE EXCEPTION 'processing group queue progress privileges are missing or excessive';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
         WHERE schemaname = 'public'
           AND tablename = 'bulk_import_jobs'
           AND policyname = 'bulk_import_jobs_processing_select'
    ) OR NOT EXISTS (
        SELECT 1 FROM pg_policies
         WHERE schemaname = 'public'
           AND tablename = 'bulk_import_jobs'
           AND policyname = 'bulk_import_jobs_processing_update'
    ) THEN
        RAISE EXCEPTION 'verified-context queue RLS policies are missing';
    END IF;

    RAISE NOTICE 'Signed bulk-import context verifier and restricted queue boundary verified; no key material was displayed.';
END
$$;

SELECT
    current_user AS inspected_as,
    to_regprocedure('stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)') IS NOT NULL AS verifier_installed,
    to_regprocedure('stonez_internal.current_bulk_import_context()') IS NOT NULL AS context_helper_installed,
    has_function_privilege('stonez_bulk_process_group', 'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,integer,text,bigint,bigint,bigint,text)', 'EXECUTE') AS process_group_can_verify,
    has_table_privilege('stonez_bulk_process_group', 'stonez_internal.bulk_context_keys', 'SELECT') AS process_group_can_read_keys;
