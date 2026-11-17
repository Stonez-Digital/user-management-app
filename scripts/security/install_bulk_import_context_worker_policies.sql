-- Install signed-context RLS policies for the isolated bulk worker.
-- Run as the owner of the listed application tables after installing the
-- private verifier functions. This does not change the API runtime policies.
BEGIN;

DO $owner_check$
DECLARE
    v_table text;
BEGIN
    FOREACH v_table IN ARRAY ARRAY[
        'bulk_import_jobs','users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections','student_enrollments',
        'guardian_relationships'
    ] LOOP
        IF NOT EXISTS (
            SELECT 1
              FROM pg_class AS relation
              JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
              JOIN pg_roles AS owner_role ON owner_role.oid = relation.relowner
             WHERE namespace.nspname = 'public'
               AND relation.relname = v_table
               AND owner_role.rolname = current_user
        ) THEN
            RAISE EXCEPTION 'current role % must own public.% before installing worker policies', current_user, v_table;
        END IF;
    END LOOP;
END
$owner_check$;

DO $public_policy_check$
DECLARE
    v_table text;
BEGIN
    FOREACH v_table IN ARRAY ARRAY[
        'bulk_import_jobs','users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections','student_enrollments',
        'guardian_relationships'
    ] LOOP
        IF EXISTS (
            SELECT 1 FROM pg_policy
             WHERE polrelid = format('public.%I', v_table)::regclass
               AND polpermissive
               AND 0::oid = ANY(polroles)
        ) THEN
            RAISE EXCEPTION 'public permissive RLS policy on public.% would bypass signed worker tenant scope', v_table;
        END IF;
    END LOOP;
END
$public_policy_check$;

ALTER TABLE public.bulk_import_jobs ENABLE ROW LEVEL SECURITY;
GRANT SELECT ON public.bulk_import_jobs TO stonez_bulk_context_owner;
GRANT UPDATE (lease_id) ON public.bulk_import_jobs TO stonez_bulk_context_owner;
GRANT SELECT ON public.bulk_import_jobs TO stonez_bulk_worker_processing;
GRANT UPDATE (status,total,valid,created,updated,skipped,failed,credentials_sent,credential_emails_failed,errors_json,claimed_at,worker_id,last_error,processed_rows_json)
    ON public.bulk_import_jobs TO stonez_bulk_worker_processing;

DROP POLICY IF EXISTS bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs
    FOR SELECT TO stonez_bulk_context_owner USING (true);
DROP POLICY IF EXISTS bulk_import_jobs_context_verifier_lock ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_context_verifier_lock ON public.bulk_import_jobs
    FOR UPDATE TO stonez_bulk_context_owner USING (true) WITH CHECK (true);
DROP POLICY IF EXISTS bulk_import_jobs_signed_worker_select ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_signed_worker_select ON public.bulk_import_jobs
    FOR SELECT TO stonez_bulk_worker_processing
    USING (
        school_id = stonez_context_private.current_worker_school_id()
        AND initiated_by = stonez_context_private.current_worker_initiator_id()
        AND id = stonez_context_private.current_worker_job_id()
    );
DROP POLICY IF EXISTS bulk_import_jobs_signed_worker_update ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_signed_worker_update ON public.bulk_import_jobs
    FOR UPDATE TO stonez_bulk_worker_processing
    USING (
        school_id = stonez_context_private.current_worker_school_id()
        AND initiated_by = stonez_context_private.current_worker_initiator_id()
        AND id = stonez_context_private.current_worker_job_id()
    )
    WITH CHECK (
        school_id = stonez_context_private.current_worker_school_id()
        AND initiated_by = stonez_context_private.current_worker_initiator_id()
        AND id = stonez_context_private.current_worker_job_id()
    );

-- The worker's row handlers only read and create tenant data. Do not grant
-- UPDATE or DELETE on these tables as part of this import capability.
DO $policies$
DECLARE
    v_table text;
    v_columns text;
    v_insert_tables text[] := ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'student_enrollments','guardian_relationships'
    ];
    v_select_tables text[] := ARRAY[
        'users','students','teacher_profiles','parent_profiles',
        'academic_sessions','school_classes','sections',
        'student_enrollments','guardian_relationships'
    ];
BEGIN
    FOREACH v_table IN ARRAY v_select_tables LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', v_table);
        CASE v_table
            WHEN 'users' THEN v_columns := 'id, school_id, email, role';
            WHEN 'students' THEN v_columns := 'id, school_id, admission_number';
            WHEN 'teacher_profiles' THEN v_columns := 'id, school_id, staff_id';
            WHEN 'parent_profiles' THEN v_columns := 'id, school_id, parent_identifier, user_id';
            WHEN 'academic_sessions' THEN v_columns := 'id, school_id, name, status';
            WHEN 'school_classes' THEN v_columns := 'id, school_id, name';
            WHEN 'sections' THEN v_columns := 'id, school_id, class_id, name';
            WHEN 'student_enrollments' THEN v_columns := 'id, school_id, student_id, academic_session_id, status';
            WHEN 'guardian_relationships' THEN v_columns := 'id, school_id, guardian_user_id, student_id';
            ELSE RAISE EXCEPTION 'no approved worker SELECT column inventory for %', v_table;
        END CASE;
        EXECUTE format('GRANT SELECT (%s) ON public.%I TO stonez_bulk_worker_processing', v_columns, v_table);
        EXECUTE format('DROP POLICY IF EXISTS bulk_worker_signed_select ON public.%I', v_table);
        EXECUTE format(
            'CREATE POLICY bulk_worker_signed_select ON public.%I FOR SELECT TO stonez_bulk_worker_processing USING (school_id = stonez_context_private.current_worker_school_id())',
            v_table
        );
    END LOOP;

    FOREACH v_table IN ARRAY v_insert_tables LOOP
        EXECUTE format('GRANT INSERT ON public.%I TO stonez_bulk_worker_processing', v_table);
        EXECUTE format('DROP POLICY IF EXISTS bulk_worker_signed_insert ON public.%I', v_table);
        EXECUTE format(
            'CREATE POLICY bulk_worker_signed_insert ON public.%I FOR INSERT TO stonez_bulk_worker_processing WITH CHECK (school_id = stonez_context_private.current_worker_school_id())',
            v_table
        );
    END LOOP;

    -- User creation is a constrained import capability, not general account
    -- provisioning. Revoke the table-level grant and grant only the columns
    -- emitted by the bulk-import User model.
    REVOKE INSERT ON public.users FROM stonez_bulk_worker_processing;
    GRANT INSERT (id, school_id, name, email, password_hash, role, active)
        ON public.users TO stonez_bulk_worker_processing;

    CREATE OR REPLACE FUNCTION stonez_context_private.bulk_worker_user_role_allowed(p_role text)
    RETURNS boolean
    LANGUAGE sql
    IMMUTABLE
    PARALLEL SAFE
    SET search_path = pg_catalog
    AS $role_guard$
        SELECT p_role IN ('student', 'teacher', 'parent')
    $role_guard$;
    REVOKE ALL ON FUNCTION stonez_context_private.bulk_worker_user_role_allowed(text) FROM PUBLIC;
    GRANT EXECUTE ON FUNCTION stonez_context_private.bulk_worker_user_role_allowed(text)
        TO stonez_bulk_worker_processing;

    -- A restrictive policy is ANDed with every permissive INSERT policy,
    -- so weakening/replacing the tenant policy cannot enable privileged roles.
    DROP POLICY IF EXISTS bulk_worker_user_role_guard ON public.users;
    CREATE POLICY bulk_worker_user_role_guard ON public.users
        AS RESTRICTIVE
        FOR INSERT TO stonez_bulk_worker_processing
        WITH CHECK (stonez_context_private.bulk_worker_user_role_allowed(role));
END
$policies$;

COMMIT;
