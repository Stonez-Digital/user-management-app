-- Read-only verifier for the bulk-import tenant RLS boundary.
-- Does not modify database state or display tenant data / signing material.
DO $verify$
DECLARE
    v_table text;
    v_role oid;
    v_policy_count integer;
BEGIN
    SELECT oid INTO v_role FROM pg_roles WHERE rolname = 'stonez_bulk_process_group';
    IF v_role IS NULL THEN RAISE EXCEPTION 'missing stonez_bulk_process_group'; END IF;
    IF to_regprocedure('stonez_internal.current_bulk_import_context()') IS NULL
       OR NOT has_function_privilege('stonez_bulk_process_group',
           'stonez_internal.current_bulk_import_context()', 'EXECUTE') THEN
        RAISE EXCEPTION 'verified context function is missing or inaccessible';
    END IF;

    FOR v_table IN SELECT unnest(ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections',
        'student_enrollments','guardian_relationships'
    ]) LOOP
        IF NOT EXISTS (
            SELECT 1 FROM pg_class c
             WHERE c.oid = to_regclass(format('public.%I', v_table))
               AND c.relrowsecurity AND c.relforcerowsecurity
        ) THEN
            RAISE EXCEPTION 'RLS is not enabled and forced on public.%', v_table;
        END IF;
        IF has_table_privilege('stonez_bulk_process_group', format('public.%I', v_table), 'UPDATE')
           OR has_table_privilege('stonez_bulk_process_group', format('public.%I', v_table), 'DELETE')
           OR has_table_privilege('stonez_bulk_process_group', format('public.%I', v_table), 'TRUNCATE')
           OR has_table_privilege('stonez_bulk_process_group', format('public.%I', v_table), 'REFERENCES')
           OR has_table_privilege('stonez_bulk_process_group', format('public.%I', v_table), 'TRIGGER') THEN
            RAISE EXCEPTION 'processing group has forbidden table privilege on public.%', v_table;
        END IF;
        IF NOT has_any_column_privilege('stonez_bulk_process_group', format('public.%I', v_table), 'SELECT') THEN
            RAISE EXCEPTION 'processing group has no SELECT columns on public.%', v_table;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM pg_policies
             WHERE schemaname='public' AND tablename=v_table
               AND policyname = v_table || '_bulk_import_tenant_guard'
               AND permissive='RESTRICTIVE' AND cmd='ALL'
               AND roles @> ARRAY['stonez_bulk_process_group']::name[]
               AND qual LIKE '%current_bulk_import_context%'
               AND with_check LIKE '%current_bulk_import_context%'
        ) THEN
            RAISE EXCEPTION 'restrictive tenant guard is missing or malformed on public.%', v_table;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM pg_policies
             WHERE schemaname='public' AND tablename=v_table
               AND policyname = v_table || '_bulk_import_tenant_select'
               AND permissive='PERMISSIVE' AND cmd='SELECT'
               AND roles @> ARRAY['stonez_bulk_process_group']::name[]
               AND qual LIKE '%current_bulk_import_context%'
        ) THEN
            RAISE EXCEPTION 'permissive SELECT policy is missing or malformed on public.%', v_table;
        END IF;
    END LOOP;

    IF has_table_privilege('stonez_bulk_process_group','public.users','INSERT')
       OR has_table_privilege('stonez_bulk_process_group','public.students','INSERT')
       OR has_table_privilege('stonez_bulk_process_group','public.teacher_profiles','INSERT')
       OR has_table_privilege('stonez_bulk_process_group','public.parent_profiles','INSERT')
       OR has_table_privilege('stonez_bulk_process_group','public.student_enrollments','INSERT')
       OR has_table_privilege('stonez_bulk_process_group','public.guardian_relationships','INSERT') THEN
        -- Table-level INSERT is intentionally not used; it must be column scoped.
        RAISE EXCEPTION 'processing group has broad table-level INSERT privilege';
    END IF;

    FOREACH v_table IN ARRAY ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'student_enrollments','guardian_relationships'
    ] LOOP
        IF NOT EXISTS (
            SELECT 1 FROM pg_policies
             WHERE schemaname='public' AND tablename=v_table
               AND policyname = v_table || '_bulk_import_tenant_insert'
               AND permissive='PERMISSIVE' AND cmd='INSERT'
               AND roles @> ARRAY['stonez_bulk_process_group']::name[]
               AND with_check LIKE '%current_bulk_import_context%'
        ) THEN
            RAISE EXCEPTION 'permissive INSERT policy is missing or malformed on public.%', v_table;
        END IF;
        IF NOT has_any_column_privilege('stonez_bulk_process_group', format('public.%I',v_table), 'INSERT') THEN
            RAISE EXCEPTION 'processing group has no INSERT columns on public.%', v_table;
        END IF;
    END LOOP;

    FOREACH v_table IN ARRAY ARRAY['academic_sessions','school_classes','sections'] LOOP
        IF has_any_column_privilege('stonez_bulk_process_group', format('public.%I',v_table), 'INSERT') THEN
            RAISE EXCEPTION 'processing group unexpectedly has INSERT columns on read-only table public.%', v_table;
        END IF;
    END LOOP;

    IF EXISTS (
        SELECT 1 FROM pg_policies
         WHERE schemaname='public' AND tablename='users'
           AND policyname='users_bulk_import_tenant_guard'
           AND (qual NOT LIKE '%school_id IS NOT NULL%' OR with_check NOT LIKE '%school_id IS NOT NULL%')
    ) THEN
        RAISE EXCEPTION 'users restrictive guard does not exclude NULL-school platform users';
    END IF;

    RAISE NOTICE 'Bulk-import tenant RLS grants, restrictive guards, and policy inventory verified.';
END
$verify$;
