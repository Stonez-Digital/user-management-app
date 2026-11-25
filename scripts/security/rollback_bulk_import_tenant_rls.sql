-- Roll back only the dedicated bulk-import tenant RLS artifacts.
-- Operator-run SQL. Requires a reviewed pre-install RLS manifest in the
-- current session; see BULK-IMPORT-TENANT-RLS-POLICY-SPEC.md.
-- This script deliberately does not drop roles, remove other policies, or
-- change ownership. It refuses drift rather than guessing what is safe.
\set ON_ERROR_STOP on
BEGIN;

DO $preflight$
DECLARE
    v_role oid;
    v_table text;
    v_baseline record;
    v_policy record;
    v_select_columns text[];
    v_insert_columns text[];
    v_expected_policy_count integer;
BEGIN
    SELECT oid INTO v_role
      FROM pg_roles
     WHERE rolname = 'stonez_bulk_process_group';
    IF v_role IS NULL THEN
        RAISE EXCEPTION 'missing stonez_bulk_process_group; refusing rollback';
    END IF;

    IF to_regclass('pg_temp.bulk_import_tenant_rls_rollback_manifest') IS NULL THEN
        RAISE EXCEPTION 'reviewed pg_temp.bulk_import_tenant_rls_rollback_manifest is required';
    END IF;
    IF (SELECT count(*) FROM pg_temp.bulk_import_tenant_rls_rollback_manifest) <> 9
       OR EXISTS (
           SELECT 1
             FROM pg_temp.bulk_import_tenant_rls_rollback_manifest
            WHERE table_name NOT IN (
                'users','students','teacher_profiles','parent_profiles',
                'academic_sessions','school_classes','sections',
                'student_enrollments','guardian_relationships'
            )
       ) THEN
        RAISE EXCEPTION 'rollback manifest must contain exactly the nine reviewed tenant tables';
    END IF;
    IF EXISTS (
        SELECT expected.table_name
          FROM (VALUES
            ('users'),('students'),('teacher_profiles'),('parent_profiles'),
            ('academic_sessions'),('school_classes'),('sections'),
            ('student_enrollments'),('guardian_relationships')
          ) AS expected(table_name)
          LEFT JOIN pg_temp.bulk_import_tenant_rls_rollback_manifest manifest
            ON manifest.table_name = expected.table_name
         WHERE manifest.table_name IS NULL
    ) THEN
        RAISE EXCEPTION 'rollback manifest is missing a required tenant table';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_temp.bulk_import_tenant_rls_rollback_manifest
         WHERE relforcerowsecurity
    ) THEN
        RAISE EXCEPTION 'manifest records FORCE RLS; this installer does not support forced-RLS rollback';
    END IF;

    FOR v_table IN SELECT unnest(ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections',
        'student_enrollments','guardian_relationships'
    ]) LOOP
        SELECT * INTO v_baseline
          FROM pg_temp.bulk_import_tenant_rls_rollback_manifest
         WHERE table_name = v_table;

        IF to_regclass(format('public.%I', v_table)) IS NULL THEN
            RAISE EXCEPTION 'required table public.% is missing; refusing partial rollback', v_table;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM pg_class c
             WHERE c.oid = to_regclass(format('public.%I', v_table))
               AND c.relrowsecurity AND NOT c.relforcerowsecurity
        ) THEN
            RAISE EXCEPTION 'public.% RLS state drifted after installation; refusing rollback', v_table;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM pg_roles r
             WHERE r.rolname = current_user AND r.rolsuper
        ) AND (SELECT relowner FROM pg_class WHERE oid = to_regclass(format('public.%I', v_table)))
              <> (SELECT oid FROM pg_roles WHERE rolname = current_user) THEN
            RAISE EXCEPTION 'rollback must run as superuser or owner of public.%', v_table;
        END IF;

        -- The installer creates only column-level grants. Any table-level
        -- grant or unexpected column privilege means state changed afterward.
        IF EXISTS (
            SELECT 1
              FROM pg_class c
              CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl, acldefault('r', c.relowner))) acl
             WHERE c.oid = to_regclass(format('public.%I', v_table))
               AND acl.grantee = v_role
        ) THEN
            RAISE EXCEPTION 'unexpected table-level ACL for processing group on public.%', v_table;
        END IF;

        v_select_columns := CASE v_table
            WHEN 'users' THEN ARRAY['id','school_id','email','role']
            WHEN 'students' THEN ARRAY['id','school_id','admission_number']
            WHEN 'teacher_profiles' THEN ARRAY['id','school_id','staff_id']
            WHEN 'parent_profiles' THEN ARRAY['id','school_id','parent_identifier','user_id']
            WHEN 'academic_sessions' THEN ARRAY['id','school_id','name','status']
            WHEN 'school_classes' THEN ARRAY['id','school_id','name']
            WHEN 'sections' THEN ARRAY['id','school_id','class_id','name']
            WHEN 'student_enrollments' THEN ARRAY['id','school_id','student_id','academic_session_id','status']
            WHEN 'guardian_relationships' THEN ARRAY['id','school_id','guardian_user_id','student_id']
        END;
        v_insert_columns := CASE v_table
            WHEN 'users' THEN ARRAY['id','school_id','name','email','password_hash','role','active']
            WHEN 'students' THEN ARRAY['id','school_id','user_id','admission_number','date_of_birth','gender','guardian_name','guardian_phone','guardian_email','enrollment_status','created_at','updated_at']
            WHEN 'teacher_profiles' THEN ARRAY['id','school_id','user_id','staff_id','phone','gender','employment_date','department','designation','subjects','classes','status','created_at','updated_at']
            WHEN 'parent_profiles' THEN ARRAY['id','school_id','user_id','parent_identifier','phone','address','occupation','created_at','updated_at']
            WHEN 'student_enrollments' THEN ARRAY['id','school_id','student_id','academic_session_id','class_id','section_id','status','enrolled_at','created_at','updated_at']
            WHEN 'guardian_relationships' THEN ARRAY['id','school_id','guardian_user_id','student_id','relationship','primary','active','created_at','updated_at']
            ELSE ARRAY[]::text[]
        END;

        IF EXISTS (
            SELECT 1
              FROM pg_attribute a
              CROSS JOIN LATERAL aclexplode(a.attacl) acl
             WHERE a.attrelid = to_regclass(format('public.%I', v_table))
               AND a.attnum > 0 AND NOT a.attisdropped
               AND acl.grantee = v_role
               AND (
                   acl.privilege_type NOT IN ('SELECT','INSERT')
                   OR (acl.privilege_type = 'SELECT' AND NOT (a.attname = ANY(v_select_columns)))
                   OR (acl.privilege_type = 'INSERT' AND NOT (a.attname = ANY(v_insert_columns)))
                   OR acl.is_grantable
               )
        ) THEN
            RAISE EXCEPTION 'unexpected column ACL for processing group on public.%', v_table;
        END IF;

        SELECT count(*) INTO v_expected_policy_count
          FROM pg_policy p
         WHERE p.polrelid = to_regclass(format('public.%I', v_table))
           AND p.polname = ANY(ARRAY[
               v_table || '_bulk_import_tenant_select',
               v_table || '_bulk_import_tenant_insert',
               v_table || '_bulk_import_tenant_guard'
           ]);
        IF v_expected_policy_count <> 3 THEN
            RAISE EXCEPTION 'expected exactly three installer-owned policies on public.%, found %', v_table, v_expected_policy_count;
        END IF;

        IF EXISTS (
            SELECT 1
              FROM pg_policy p
             WHERE p.polrelid = to_regclass(format('public.%I', v_table))
               AND p.polname = ANY(ARRAY[
                   v_table || '_bulk_import_tenant_select',
                   v_table || '_bulk_import_tenant_insert',
                   v_table || '_bulk_import_tenant_guard'
               ])
               AND (
                   p.polroles <> ARRAY[v_role]::oid[]
                   OR position('current_bulk_import_context' IN lower(
                       COALESCE(pg_get_expr(p.polqual, p.polrelid), '') || ' ' ||
                       COALESCE(pg_get_expr(p.polwithcheck, p.polrelid), '')
                   )) = 0
                   OR position('school_id' IN lower(
                       COALESCE(pg_get_expr(p.polqual, p.polrelid), '') || ' ' ||
                       COALESCE(pg_get_expr(p.polwithcheck, p.polrelid), '')
                   )) = 0
                   OR (p.polname = v_table || '_bulk_import_tenant_select'
                       AND (p.polcmd <> 'r' OR NOT p.polpermissive OR p.polqual IS NULL OR p.polwithcheck IS NOT NULL))
                   OR (p.polname = v_table || '_bulk_import_tenant_insert'
                       AND (p.polcmd <> 'a' OR NOT p.polpermissive OR p.polqual IS NOT NULL OR p.polwithcheck IS NULL))
                   OR (p.polname = v_table || '_bulk_import_tenant_guard'
                       AND (p.polcmd <> '*' OR p.polpermissive OR p.polqual IS NULL OR p.polwithcheck IS NULL))
               )
        ) THEN
            RAISE EXCEPTION 'installer-owned policy drift detected on public.%; refusing rollback', v_table;
        END IF;

        IF v_table = 'users' AND EXISTS (
            SELECT 1 FROM pg_policy p
             WHERE p.polrelid = to_regclass('public.users')
               AND p.polname = 'users_bulk_import_tenant_guard'
               AND (
                   position('is not null' IN lower(pg_get_expr(p.polqual, p.polrelid))) = 0
                   OR position('is not null' IN lower(pg_get_expr(p.polwithcheck, p.polrelid))) = 0
               )
        ) THEN
            RAISE EXCEPTION 'users NULL-school guard drift detected; refusing rollback';
        END IF;
    END LOOP;
END
$preflight$;

DO $rollback$
DECLARE
    v_table text;
    v_baseline record;
BEGIN
    FOR v_table IN SELECT unnest(ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections',
        'student_enrollments','guardian_relationships'
    ]) LOOP
        SELECT * INTO v_baseline
          FROM pg_temp.bulk_import_tenant_rls_rollback_manifest
         WHERE table_name = v_table;

        EXECUTE format('DROP POLICY %I ON public.%I', v_table || '_bulk_import_tenant_select', v_table);
        EXECUTE format('DROP POLICY %I ON public.%I', v_table || '_bulk_import_tenant_insert', v_table);
        EXECUTE format('DROP POLICY %I ON public.%I', v_table || '_bulk_import_tenant_guard', v_table);

        -- Revoke only the column grants this installer adds. Other policies,
        -- roles, memberships, ownership and grants are deliberately untouched.
        CASE v_table
            WHEN 'users' THEN
                REVOKE SELECT (id, school_id, email, role), INSERT (id, school_id, name, email, password_hash, role, active) ON TABLE public.users FROM stonez_bulk_process_group;
            WHEN 'students' THEN
                REVOKE SELECT (id, school_id, admission_number), INSERT (id, school_id, user_id, admission_number, date_of_birth, gender, guardian_name, guardian_phone, guardian_email, enrollment_status, created_at, updated_at) ON TABLE public.students FROM stonez_bulk_process_group;
            WHEN 'teacher_profiles' THEN
                REVOKE SELECT (id, school_id, staff_id), INSERT (id, school_id, user_id, staff_id, phone, gender, employment_date, department, designation, subjects, classes, status, created_at, updated_at) ON TABLE public.teacher_profiles FROM stonez_bulk_process_group;
            WHEN 'parent_profiles' THEN
                REVOKE SELECT (id, school_id, parent_identifier, user_id), INSERT (id, school_id, user_id, parent_identifier, phone, address, occupation, created_at, updated_at) ON TABLE public.parent_profiles FROM stonez_bulk_process_group;
            WHEN 'academic_sessions' THEN
                REVOKE SELECT (id, school_id, name, status) ON TABLE public.academic_sessions FROM stonez_bulk_process_group;
            WHEN 'school_classes' THEN
                REVOKE SELECT (id, school_id, name) ON TABLE public.school_classes FROM stonez_bulk_process_group;
            WHEN 'sections' THEN
                REVOKE SELECT (id, school_id, class_id, name) ON TABLE public.sections FROM stonez_bulk_process_group;
            WHEN 'student_enrollments' THEN
                REVOKE SELECT (id, school_id, student_id, academic_session_id, status), INSERT (id, school_id, student_id, academic_session_id, class_id, section_id, status, enrolled_at, created_at, updated_at) ON TABLE public.student_enrollments FROM stonez_bulk_process_group;
            WHEN 'guardian_relationships' THEN
                REVOKE SELECT (id, school_id, guardian_user_id, student_id), INSERT (id, school_id, guardian_user_id, student_id, relationship, "primary", active, created_at, updated_at) ON TABLE public.guardian_relationships FROM stonez_bulk_process_group;
        END CASE;

        IF NOT v_baseline.relrowsecurity THEN
            EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY', v_table);
        END IF;
    END LOOP;
END
$rollback$;

COMMIT;
