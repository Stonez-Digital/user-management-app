-- Read-only role-preparation assertions. Run after provisioning but BEFORE
-- install_bulk_import_claim_function.sql, using a role that can inspect ACLs.
DO $$
DECLARE
    owner_role pg_roles%ROWTYPE;
    api_role pg_roles%ROWTYPE;
BEGIN
    SELECT * INTO owner_role FROM pg_roles WHERE rolname = 'stonez_bulk_queue_owner';
    IF NOT FOUND THEN RAISE EXCEPTION 'missing stonez_bulk_queue_owner'; END IF;
    SELECT * INTO api_role FROM pg_roles WHERE rolname = 'stonez_api_runtime';
    IF NOT FOUND THEN RAISE EXCEPTION 'missing stonez_api_runtime'; END IF;
    IF owner_role.rolcanlogin OR owner_role.rolsuper OR NOT owner_role.rolbypassrls
       OR owner_role.rolcreatedb OR owner_role.rolcreaterole OR owner_role.rolinherit THEN
        RAISE EXCEPTION 'queue owner role attributes are unsafe';
    END IF;
    IF api_role.rolcanlogin OR api_role.rolsuper OR api_role.rolbypassrls
       OR api_role.rolcreatedb OR api_role.rolcreaterole OR NOT api_role.rolinherit THEN
        RAISE EXCEPTION 'API runtime role attributes are unsafe';
    END IF;
    IF NOT has_schema_privilege('stonez_bulk_queue_owner', 'public', 'USAGE')
       OR NOT has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'SELECT') THEN
        RAISE EXCEPTION 'queue owner is missing schema/table read privileges';
    END IF;
    IF NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'status', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'worker_id', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'claimed_at', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'attempts', 'UPDATE')
       OR NOT has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'last_error', 'UPDATE') THEN
        RAISE EXCEPTION 'queue owner is missing claim-column UPDATE privileges';
    END IF;
    IF has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'id', 'UPDATE')
       OR has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'school_id', 'UPDATE')
       OR has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'initiated_by', 'UPDATE')
       OR has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'rows_json', 'UPDATE')
       OR has_column_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'errors_json', 'UPDATE') THEN
        RAISE EXCEPTION 'queue owner can update protected/non-claim columns';
    END IF;
    IF has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'UPDATE')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'INSERT')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'DELETE')
       OR has_table_privilege('stonez_bulk_queue_owner', 'public.bulk_import_jobs', 'TRUNCATE') THEN
        RAISE EXCEPTION 'queue owner has unexpected write privileges';
    END IF;
    IF has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'SELECT')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'INSERT')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'UPDATE')
       OR has_any_column_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'REFERENCES')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'TRUNCATE')
       OR has_table_privilege('stonez_api_runtime', 'public.bulk_import_jobs', 'TRIGGER') THEN
        RAISE EXCEPTION 'API runtime has direct queue-table privileges';
    END IF;
    IF pg_has_role('stonez_api_runtime', 'stonez_bulk_queue_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'API runtime must not be a member of queue owner';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_roles AS privileged
        WHERE (privileged.rolsuper OR privileged.rolbypassrls)
          AND privileged.rolname NOT IN ('stonez_api_runtime', 'stonez_bulk_queue_owner')
          AND pg_has_role('stonez_api_runtime', privileged.oid, 'MEMBER')
    ) THEN
        RAISE EXCEPTION 'API runtime is a member of another privileged role';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_roles AS privileged
        WHERE (privileged.rolsuper OR privileged.rolbypassrls)
          AND privileged.rolname NOT IN ('stonez_bulk_queue_owner')
          AND pg_has_role('stonez_bulk_queue_owner', privileged.oid, 'MEMBER')
    ) THEN
        RAISE EXCEPTION 'queue owner is a member of another privileged role';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.table_privileges
        WHERE grantee = 'stonez_bulk_queue_owner'
          AND table_schema = 'public'
          AND table_name <> 'bulk_import_jobs'
    ) THEN
        RAISE EXCEPTION 'queue owner has unexpected privileges on another public table';
    END IF;
END
$$;
