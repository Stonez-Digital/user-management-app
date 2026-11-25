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
        IF (SELECT relowner FROM pg_class WHERE oid = to_regclass(format('public.%I', v_table))) = v_role.oid THEN
            RAISE EXCEPTION 'processing group must not own public.%', v_table;
        END IF;
    END LOOP;
END
$preflight$;

-- Remove only this installer's named policies, so reruns are deterministic.
DROP POLICY IF EXISTS users_bulk_import_tenant_select ON public.users;
DROP POLICY IF EXISTS users_bulk_import_tenant_insert ON public.users;
DROP POLICY IF EXISTS users_bulk_import_tenant_guard ON public.users;
DROP POLICY IF EXISTS students_bulk_import_tenant_select ON public.students;
DROP POLICY IF EXISTS students_bulk_import_tenant_insert ON public.students;
DROP POLICY IF EXISTS students_bulk_import_tenant_guard ON public.students;
DROP POLICY IF EXISTS teacher_profiles_bulk_import_tenant_select ON public.teacher_profiles;
DROP POLICY IF EXISTS teacher_profiles_bulk_import_tenant_insert ON public.teacher_profiles;
DROP POLICY IF EXISTS teacher_profiles_bulk_import_tenant_guard ON public.teacher_profiles;
DROP POLICY IF EXISTS parent_profiles_bulk_import_tenant_select ON public.parent_profiles;
DROP POLICY IF EXISTS parent_profiles_bulk_import_tenant_insert ON public.parent_profiles;
DROP POLICY IF EXISTS parent_profiles_bulk_import_tenant_guard ON public.parent_profiles;
DROP POLICY IF EXISTS academic_sessions_bulk_import_tenant_select ON public.academic_sessions;
DROP POLICY IF EXISTS academic_sessions_bulk_import_tenant_guard ON public.academic_sessions;
DROP POLICY IF EXISTS school_classes_bulk_import_tenant_select ON public.school_classes;
DROP POLICY IF EXISTS school_classes_bulk_import_tenant_guard ON public.school_classes;
DROP POLICY IF EXISTS sections_bulk_import_tenant_select ON public.sections;
DROP POLICY IF EXISTS sections_bulk_import_tenant_guard ON public.sections;
DROP POLICY IF EXISTS student_enrollments_bulk_import_tenant_select ON public.student_enrollments;
DROP POLICY IF EXISTS student_enrollments_bulk_import_tenant_insert ON public.student_enrollments;
DROP POLICY IF EXISTS student_enrollments_bulk_import_tenant_guard ON public.student_enrollments;
DROP POLICY IF EXISTS guardian_relationships_bulk_import_tenant_select ON public.guardian_relationships;
DROP POLICY IF EXISTS guardian_relationships_bulk_import_tenant_insert ON public.guardian_relationships;
DROP POLICY IF EXISTS guardian_relationships_bulk_import_tenant_guard ON public.guardian_relationships;

-- Clear existing direct grants on this dedicated group, including column ACLs.
DO $privileges$
DECLARE
    v_table text;
    v_column text;
BEGIN
    FOREACH v_table IN ARRAY ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections',
        'student_enrollments','guardian_relationships'
    ] LOOP
        EXECUTE format('REVOKE ALL PRIVILEGES ON TABLE public.%I FROM stonez_bulk_process_group', v_table);
        FOR v_column IN
            SELECT attname FROM pg_attribute
             WHERE attrelid = to_regclass(format('public.%I', v_table))
               AND attnum > 0 AND NOT attisdropped
        LOOP
            EXECUTE format('REVOKE ALL PRIVILEGES (%I) ON TABLE public.%I FROM stonez_bulk_process_group', v_column, v_table);
        END LOOP;
    END LOOP;
END
$privileges$;

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
GRANT INSERT (id, school_id, guardian_user_id, student_id, relationship, primary,
    active, created_at, updated_at)
    ON TABLE public.guardian_relationships TO stonez_bulk_process_group;

-- FORCE RLS so table owners cannot accidentally bypass the tenant policies.
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.users FORCE ROW LEVEL SECURITY;
ALTER TABLE public.students ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.students FORCE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE public.parent_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.parent_profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE public.academic_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.academic_sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE public.school_classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.school_classes FORCE ROW LEVEL SECURITY;
ALTER TABLE public.sections ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sections FORCE ROW LEVEL SECURITY;
ALTER TABLE public.student_enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.student_enrollments FORCE ROW LEVEL SECURITY;
ALTER TABLE public.guardian_relationships ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.guardian_relationships FORCE ROW LEVEL SECURITY;

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
