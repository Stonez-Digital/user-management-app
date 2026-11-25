-- Install the dedicated bulk-import tenant RLS boundary.
-- Operator-run SQL. Requires the signed-context installer to have run first.
-- This is intentionally scoped to stonez_bulk_process_group and the nine
-- reviewed tenant-content tables. It does not provision roles or credentials.
\set ON_ERROR_STOP on
BEGIN;

DO $preflight$
DECLARE
    v_role pg_roles%ROWTYPE;
    v_table text;
BEGIN
    SELECT * INTO v_role FROM pg_roles WHERE rolname = 'stonez_bulk_process_group';
    IF NOT FOUND THEN RAISE EXCEPTION 'missing stonez_bulk_process_group'; END IF;
    IF v_role.rolcanlogin OR v_role.rolsuper OR v_role.rolbypassrls
       OR v_role.rolcreatedb OR v_role.rolcreaterole OR NOT v_role.rolinherit THEN
        RAISE EXCEPTION 'bulk-import processing group role attributes are unsafe';
    END IF;
    IF to_regprocedure('stonez_internal.current_bulk_import_context()') IS NULL THEN
        RAISE EXCEPTION 'verified bulk-import context function is missing';
    END IF;
    IF NOT has_function_privilege('stonez_bulk_process_group',
        'stonez_internal.current_bulk_import_context()', 'EXECUTE') THEN
        RAISE EXCEPTION 'processing group cannot execute verified context function';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_auth_members
         WHERE member = v_role.oid
    ) THEN
        RAISE EXCEPTION 'processing group must not inherit any other role';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_auth_members membership
          JOIN pg_roles member_role ON member_role.oid = membership.member
         WHERE membership.roleid = v_role.oid
           AND membership.member <> (SELECT oid FROM pg_roles WHERE rolname = current_user)
           AND (NOT member_role.rolcanlogin OR member_role.rolsuper OR member_role.rolbypassrls
                OR member_role.rolcreatedb OR member_role.rolcreaterole
                OR membership.admin_option OR NOT membership.inherit_option OR membership.set_option)
    ) THEN
        RAISE EXCEPTION 'processing group has an unsafe non-operator login membership';
    END IF;
    FOREACH v_table IN ARRAY ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections',
        'student_enrollments','guardian_relationships'
    ] LOOP
        IF to_regclass(format('public.%I', v_table)) IS NULL THEN
            RAISE EXCEPTION 'required tenant table public.% is missing', v_table;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM pg_attribute a
            WHERE a.attrelid = to_regclass(format('public.%I', v_table))
              AND a.attname = 'school_id' AND a.attnum > 0 AND NOT a.attisdropped
        ) THEN
            RAISE EXCEPTION 'required school_id column is missing on public.%', v_table;
        END IF;
        IF EXISTS (
            SELECT 1 FROM pg_class c
             WHERE c.oid = to_regclass(format('public.%I', v_table))
               AND c.relforcerowsecurity
        ) THEN
            RAISE EXCEPTION 'public.% already has FORCE ROW LEVEL SECURITY; installer requires a reviewed baseline', v_table;
        END IF;
        IF EXISTS (
            SELECT 1 FROM pg_policy p
             WHERE p.polrelid = to_regclass(format('public.%I', v_table))
               AND p.polname = ANY (ARRAY[
                   v_table || '_bulk_import_tenant_select',
                   v_table || '_bulk_import_tenant_insert',
                   v_table || '_bulk_import_tenant_guard'
               ])
        ) THEN
            RAISE EXCEPTION 'reserved bulk-import policy name already exists on public.%', v_table;
        END IF;
        IF EXISTS (
            SELECT 1
              FROM pg_class c
              CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl, acldefault('r', c.relowner))) acl
             WHERE c.oid = to_regclass(format('public.%I', v_table))
               AND acl.grantee = 0
        ) OR EXISTS (
            SELECT 1
              FROM pg_attribute a
              CROSS JOIN LATERAL aclexplode(a.attacl) acl
             WHERE a.attrelid = to_regclass(format('public.%I', v_table))
               AND a.attnum > 0 AND NOT a.attisdropped
               AND acl.grantee = 0
        ) THEN
            RAISE EXCEPTION 'PUBLIC has pre-existing direct table/column ACLs on public.%; refusing globally effective access', v_table;
        END IF;
        IF EXISTS (
            SELECT 1
              FROM pg_class c
              CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl, acldefault('r', c.relowner))) acl
             WHERE c.oid = to_regclass(format('public.%I', v_table))
               AND acl.grantee = v_role.oid
        ) OR EXISTS (
            SELECT 1
              FROM pg_attribute a
              CROSS JOIN LATERAL aclexplode(a.attacl) acl
             WHERE a.attrelid = to_regclass(format('public.%I', v_table))
               AND a.attnum > 0 AND NOT a.attisdropped
               AND acl.grantee = v_role.oid
        ) THEN
            RAISE EXCEPTION 'processing group has pre-existing direct ACLs on public.%; refusing to overwrite state', v_table;
        END IF;
        IF (SELECT relowner FROM pg_class WHERE oid = to_regclass(format('public.%I', v_table))) = v_role.oid THEN
            RAISE EXCEPTION 'processing group must not own public.%', v_table;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = current_user AND rolsuper)
           AND (SELECT relowner FROM pg_class WHERE oid = to_regclass(format('public.%I', v_table)))
               <> (SELECT oid FROM pg_roles WHERE rolname = current_user) THEN
            RAISE EXCEPTION 'installer must run as superuser or owner of public.%', v_table;
        END IF;
    END LOOP;
END
$preflight$;

-- The preflight requires a clean baseline; no pre-existing policy is replaced.

-- No ACLs are revoked: preflight proved this role has no direct grants on these tables.

GRANT SELECT (id, school_id, email, role)
    ON TABLE public.users TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, admission_number)
    ON TABLE public.students TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, staff_id)
    ON TABLE public.teacher_profiles TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, parent_identifier, user_id)
    ON TABLE public.parent_profiles TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, name, status)
    ON TABLE public.academic_sessions TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, name)
    ON TABLE public.school_classes TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, class_id, name)
    ON TABLE public.sections TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, student_id, academic_session_id, status)
    ON TABLE public.student_enrollments TO stonez_bulk_process_group;
GRANT SELECT (id, school_id, guardian_user_id, student_id)
    ON TABLE public.guardian_relationships TO stonez_bulk_process_group;

GRANT INSERT (id, school_id, name, email, password_hash, role, active)
    ON TABLE public.users TO stonez_bulk_process_group;
GRANT INSERT (id, school_id, user_id, admission_number, date_of_birth, gender,
    guardian_name, guardian_phone, guardian_email, enrollment_status, created_at, updated_at)
    ON TABLE public.students TO stonez_bulk_process_group;
GRANT INSERT (id, school_id, user_id, staff_id, phone, gender, employment_date,
    department, designation, subjects, classes, status, created_at, updated_at)
    ON TABLE public.teacher_profiles TO stonez_bulk_process_group;
GRANT INSERT (id, school_id, user_id, parent_identifier, phone, address,
    occupation, created_at, updated_at)
    ON TABLE public.parent_profiles TO stonez_bulk_process_group;
GRANT INSERT (id, school_id, student_id, academic_session_id, class_id, section_id,
    status, enrolled_at, created_at, updated_at)
    ON TABLE public.student_enrollments TO stonez_bulk_process_group;
GRANT INSERT (id, school_id, guardian_user_id, student_id, relationship, "primary",
    active, created_at, updated_at)
    ON TABLE public.guardian_relationships TO stonez_bulk_process_group;

-- Keep RLS enabled but not forced: preserve application table-owner behavior.\n-- The dedicated non-owner processing role remains subject to these policies.
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.students ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.parent_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.academic_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.school_classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sections ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.student_enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.guardian_relationships ENABLE ROW LEVEL SECURITY;

-- Restrictive guard composes with every permissive policy (AND vs OR).
-- It also blocks NULL-school platform accounts on users.
CREATE POLICY users_bulk_import_tenant_guard ON public.users
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (school_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx
         WHERE ctx.school_id = users.school_id
    ))
    WITH CHECK (school_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx
         WHERE ctx.school_id = users.school_id
    ));

CREATE POLICY users_bulk_import_tenant_select ON public.users
    FOR SELECT TO stonez_bulk_process_group
    USING (school_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx
         WHERE ctx.school_id = users.school_id
    ));
CREATE POLICY users_bulk_import_tenant_insert ON public.users
    FOR INSERT TO stonez_bulk_process_group
    WITH CHECK (school_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx
         WHERE ctx.school_id = users.school_id
    ));

CREATE POLICY students_bulk_import_tenant_guard ON public.students
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = students.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = students.school_id));
CREATE POLICY students_bulk_import_tenant_select ON public.students
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = students.school_id));
CREATE POLICY students_bulk_import_tenant_insert ON public.students
    FOR INSERT TO stonez_bulk_process_group
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = students.school_id));

CREATE POLICY teacher_profiles_bulk_import_tenant_guard ON public.teacher_profiles
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = teacher_profiles.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = teacher_profiles.school_id));
CREATE POLICY teacher_profiles_bulk_import_tenant_select ON public.teacher_profiles
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = teacher_profiles.school_id));
CREATE POLICY teacher_profiles_bulk_import_tenant_insert ON public.teacher_profiles
    FOR INSERT TO stonez_bulk_process_group
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = teacher_profiles.school_id));

CREATE POLICY parent_profiles_bulk_import_tenant_guard ON public.parent_profiles
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = parent_profiles.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = parent_profiles.school_id));
CREATE POLICY parent_profiles_bulk_import_tenant_select ON public.parent_profiles
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = parent_profiles.school_id));
CREATE POLICY parent_profiles_bulk_import_tenant_insert ON public.parent_profiles
    FOR INSERT TO stonez_bulk_process_group
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = parent_profiles.school_id));

CREATE POLICY academic_sessions_bulk_import_tenant_guard ON public.academic_sessions
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = academic_sessions.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = academic_sessions.school_id));
CREATE POLICY academic_sessions_bulk_import_tenant_select ON public.academic_sessions
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = academic_sessions.school_id));

CREATE POLICY school_classes_bulk_import_tenant_guard ON public.school_classes
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = school_classes.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = school_classes.school_id));
CREATE POLICY school_classes_bulk_import_tenant_select ON public.school_classes
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = school_classes.school_id));

CREATE POLICY sections_bulk_import_tenant_guard ON public.sections
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = sections.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = sections.school_id));
CREATE POLICY sections_bulk_import_tenant_select ON public.sections
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = sections.school_id));

CREATE POLICY student_enrollments_bulk_import_tenant_guard ON public.student_enrollments
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = student_enrollments.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = student_enrollments.school_id));
CREATE POLICY student_enrollments_bulk_import_tenant_select ON public.student_enrollments
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = student_enrollments.school_id));
CREATE POLICY student_enrollments_bulk_import_tenant_insert ON public.student_enrollments
    FOR INSERT TO stonez_bulk_process_group
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = student_enrollments.school_id));

CREATE POLICY guardian_relationships_bulk_import_tenant_guard ON public.guardian_relationships
    AS RESTRICTIVE FOR ALL TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = guardian_relationships.school_id))
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = guardian_relationships.school_id));
CREATE POLICY guardian_relationships_bulk_import_tenant_select ON public.guardian_relationships
    FOR SELECT TO stonez_bulk_process_group
    USING (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = guardian_relationships.school_id));
CREATE POLICY guardian_relationships_bulk_import_tenant_insert ON public.guardian_relationships
    FOR INSERT TO stonez_bulk_process_group
    WITH CHECK (EXISTS (SELECT 1 FROM stonez_internal.current_bulk_import_context() ctx WHERE ctx.school_id = guardian_relationships.school_id));

COMMIT;
