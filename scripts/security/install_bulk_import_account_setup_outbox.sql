-- Install the private, signed-context-bound account-setup notification outbox.
-- Run only after the signed-context verifier and worker RLS policy installers.
-- Operator-run only: this is not an application migration.
BEGIN;

DO $preflight$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='stonez_bulk_context_owner' AND NOT rolcanlogin AND NOT rolinherit)
       OR NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='stonez_bulk_worker_processing' AND NOT rolcanlogin)
       OR NOT EXISTS (
           SELECT 1 FROM pg_namespace
            WHERE nspname='stonez_context_private'
              AND nspowner=(SELECT oid FROM pg_roles WHERE rolname='stonez_bulk_context_owner')
       ) THEN
        RAISE EXCEPTION 'signed-context owner, processing group, and private schema must be installed first';
    END IF;
    IF NOT has_schema_privilege('stonez_bulk_worker_processing','stonez_context_private','USAGE') THEN
        RAISE EXCEPTION 'processing group must have private schema USAGE';
    END IF;
    IF to_regclass('public.bulk_import_jobs') IS NULL OR to_regclass('public.users') IS NULL THEN
        RAISE EXCEPTION 'public.bulk_import_jobs and public.users are required';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_class
         WHERE oid='public.users'::regclass
           AND relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)
    ) THEN
        RAISE EXCEPTION 'current role % must own public.users to install the restricted identity-read policy',current_user;
    END IF;
END
$preflight$;

GRANT stonez_bulk_context_owner TO CURRENT_USER WITH INHERIT FALSE, SET TRUE;
GRANT USAGE ON SCHEMA public TO stonez_bulk_context_owner;
GRANT SELECT (id,school_id,email,name,role) ON public.users TO stonez_bulk_context_owner;
DROP POLICY IF EXISTS bulk_worker_outbox_owner_user_read ON public.users;
CREATE POLICY bulk_worker_outbox_owner_user_read ON public.users
    FOR SELECT TO stonez_bulk_context_owner
    USING (school_id = stonez_context_private.current_worker_school_id());

SET LOCAL ROLE stonez_bulk_context_owner;

CREATE TABLE IF NOT EXISTS stonez_context_private.account_setup_outbox (
    event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL,
    job_id uuid NOT NULL,
    user_id uuid NOT NULL,
    recipient_email text NOT NULL CHECK (recipient_email <> '' AND length(recipient_email) <= 320),
    display_name text NOT NULL DEFAULT '' CHECK (length(display_name) <= 200),
    account_role text NOT NULL CHECK (account_role IN ('student','teacher','parent')),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','delivering','delivered','failed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0 AND attempt_count <= 8),
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    delivery_lease uuid,
    lease_expires_at timestamptz,
    provider_message_id text,
    failure_code text NOT NULL DEFAULT ''
        CHECK (failure_code IN ('','mailer_unconfigured','provider_rejected','provider_unavailable','provider_timeout')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    delivered_at timestamptz,
    CONSTRAINT account_setup_outbox_job_user_unique UNIQUE (job_id,user_id),
    CONSTRAINT account_setup_outbox_lease_pair CHECK (
        (delivery_lease IS NULL AND lease_expires_at IS NULL)
        OR (delivery_lease IS NOT NULL AND lease_expires_at IS NOT NULL)
    ),
    CONSTRAINT account_setup_outbox_delivery_state CHECK (
        (status='delivered' AND delivered_at IS NOT NULL AND delivery_lease IS NULL)
        OR (status<>'delivered' AND delivered_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS account_setup_outbox_due_idx
    ON stonez_context_private.account_setup_outbox(school_id,job_id,next_attempt_at,created_at)
    WHERE status IN ('pending','delivering');

REVOKE ALL ON stonez_context_private.account_setup_outbox
    FROM PUBLIC,stonez_bulk_worker_processing,stonez_bulk_context_key_admin;
GRANT SELECT,INSERT,UPDATE ON stonez_context_private.account_setup_outbox
    TO stonez_bulk_context_owner;

CREATE OR REPLACE FUNCTION stonez_context_private.enqueue_account_setup_notification(
    p_user_id uuid,p_email text,p_name text,p_role text
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path=pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
    v_initiator uuid;
    v_send boolean;
    v_email text;
    v_name text;
    v_role text;
    v_event uuid;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    v_initiator := stonez_context_private.current_worker_initiator_id();
    IF v_school IS NULL OR v_job IS NULL OR v_initiator IS NULL OR p_user_id IS NULL THEN
        RAISE EXCEPTION 'verified worker context and user ID are required' USING ERRCODE='42501';
    END IF;
    SELECT send_credentials INTO v_send
      FROM public.bulk_import_jobs
     WHERE id=v_job AND school_id=v_school AND initiated_by=v_initiator;
    IF NOT FOUND OR NOT v_send THEN
        RAISE EXCEPTION 'verified import job is not authorized for account-setup notifications' USING ERRCODE='42501';
    END IF;
    SELECT email,name,role INTO v_email,v_name,v_role
      FROM public.users
     WHERE id=p_user_id AND school_id=v_school;
    IF NOT FOUND OR v_email IS NULL OR btrim(v_email)=''
       OR lower(v_email)<>lower(COALESCE(p_email,''))
       OR v_role IS DISTINCT FROM p_role
       OR v_role NOT IN ('student','teacher','parent')
       OR length(COALESCE(p_name,''))>200 THEN
        RAISE EXCEPTION 'user identity does not match the verified school and account role' USING ERRCODE='22023';
    END IF;

    INSERT INTO stonez_context_private.account_setup_outbox
        (school_id,job_id,user_id,recipient_email,display_name,account_role)
    VALUES (v_school,v_job,p_user_id,v_email,COALESCE(NULLIF(btrim(v_name),''),''),v_role)
    ON CONFLICT(job_id,user_id) DO NOTHING
    RETURNING event_id INTO v_event;
    IF v_event IS NULL THEN
        SELECT event_id INTO v_event
          FROM stonez_context_private.account_setup_outbox
         WHERE school_id=v_school AND job_id=v_job AND user_id=p_user_id;
    END IF;
    RETURN v_event;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.claim_account_setup_notification()
RETURNS TABLE(
    event_id uuid,user_id uuid,recipient_email text,display_name text,
    account_role text,attempt_count integer,idempotency_key text,delivery_lease uuid
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path=pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
    v_event uuid;
    v_lease uuid;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    IF v_school IS NULL OR v_job IS NULL THEN
        RAISE EXCEPTION 'verified worker context is required' USING ERRCODE='42501';
    END IF;
    UPDATE stonez_context_private.account_setup_outbox AS expired
       SET status='failed',delivery_lease=NULL,lease_expires_at=NULL,
           failure_code='provider_timeout',updated_at=clock_timestamp()
     WHERE expired.school_id=v_school AND expired.job_id=v_job
       AND expired.status='delivering' AND expired.lease_expires_at<=clock_timestamp()
       AND expired.attempt_count>=8;

    SELECT outbox.event_id INTO v_event
      FROM stonez_context_private.account_setup_outbox AS outbox
     WHERE outbox.school_id=v_school AND outbox.job_id=v_job
       AND outbox.attempt_count<8
       AND (
           (outbox.status='pending' AND outbox.next_attempt_at<=clock_timestamp())
           OR (outbox.status='delivering' AND outbox.lease_expires_at<=clock_timestamp())
       )
     ORDER BY outbox.created_at,outbox.event_id
     LIMIT 1 FOR UPDATE SKIP LOCKED;
    IF v_event IS NULL THEN RETURN; END IF;

    v_lease := gen_random_uuid();
    RETURN QUERY
    UPDATE stonez_context_private.account_setup_outbox AS outbox
       SET status='delivering',attempt_count=outbox.attempt_count+1,
           delivery_lease=v_lease,
           lease_expires_at=clock_timestamp()+interval '2 minutes',
           updated_at=clock_timestamp(),failure_code=''
     WHERE outbox.event_id=v_event
    RETURNING outbox.event_id,outbox.user_id,outbox.recipient_email,
              outbox.display_name,outbox.account_role,outbox.attempt_count,
              outbox.event_id::text,outbox.delivery_lease;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.complete_account_setup_notification(
    p_event_id uuid,p_delivery_lease uuid,p_provider_message_id text DEFAULT NULL
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path=pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    IF v_school IS NULL OR v_job IS NULL OR p_event_id IS NULL OR p_delivery_lease IS NULL THEN
        RAISE EXCEPTION 'verified worker context, event ID, and delivery lease are required' USING ERRCODE='42501';
    END IF;
    UPDATE stonez_context_private.account_setup_outbox
       SET status='delivered',delivered_at=clock_timestamp(),
           provider_message_id=left(COALESCE(p_provider_message_id,''),200),
           delivery_lease=NULL,lease_expires_at=NULL,failure_code='',
           updated_at=clock_timestamp()
     WHERE event_id=p_event_id AND school_id=v_school AND job_id=v_job
       AND status='delivering' AND delivery_lease=p_delivery_lease
       AND lease_expires_at>clock_timestamp();
    RETURN FOUND;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.retry_account_setup_notification(
    p_event_id uuid,p_delivery_lease uuid,p_failure_code text,p_delay_seconds integer
)
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path=pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
    v_attempt integer;
    v_status text;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    IF v_school IS NULL OR v_job IS NULL OR p_event_id IS NULL OR p_delivery_lease IS NULL THEN
        RAISE EXCEPTION 'verified worker context, event ID, and delivery lease are required' USING ERRCODE='42501';
    END IF;
    IF p_failure_code IS NULL
       OR p_failure_code NOT IN ('mailer_unconfigured','provider_rejected','provider_unavailable','provider_timeout')
       OR p_delay_seconds IS NULL OR p_delay_seconds<1 OR p_delay_seconds>3600 THEN
        RAISE EXCEPTION 'invalid retry classification or delay' USING ERRCODE='22023';
    END IF;
    SELECT attempt_count INTO v_attempt
      FROM stonez_context_private.account_setup_outbox
     WHERE event_id=p_event_id AND school_id=v_school AND job_id=v_job
       AND status='delivering' AND delivery_lease=p_delivery_lease
       AND lease_expires_at>clock_timestamp()
     FOR UPDATE;
    IF NOT FOUND THEN RETURN 'stale_lease'; END IF;
    IF v_attempt>=8 OR p_failure_code IN ('mailer_unconfigured','provider_rejected') THEN
        v_status := 'failed';
        UPDATE stonez_context_private.account_setup_outbox
           SET status=v_status,delivery_lease=NULL,lease_expires_at=NULL,
               failure_code=p_failure_code,updated_at=clock_timestamp()
         WHERE event_id=p_event_id;
    ELSE
        v_status := 'pending';
        UPDATE stonez_context_private.account_setup_outbox
           SET status=v_status,next_attempt_at=clock_timestamp()+make_interval(secs=>p_delay_seconds),
               delivery_lease=NULL,lease_expires_at=NULL,failure_code=p_failure_code,
               updated_at=clock_timestamp()
         WHERE event_id=p_event_id;
    END IF;
    RETURN v_status;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.account_setup_notification_counts()
RETURNS TABLE(delivered_count integer,failed_count integer,pending_count integer)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path=pg_catalog
AS $fn$
SELECT
    count(*) FILTER(WHERE status='delivered')::integer,
    count(*) FILTER(WHERE status='failed')::integer,
    count(*) FILTER(WHERE status IN ('pending','delivering'))::integer
FROM stonez_context_private.account_setup_outbox
WHERE school_id=stonez_context_private.current_worker_school_id()
  AND job_id=stonez_context_private.current_worker_job_id()
$fn$;

REVOKE ALL ON FUNCTION stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.claim_account_setup_notification() FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.complete_account_setup_notification(uuid,uuid,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.account_setup_notification_counts() FROM PUBLIC;

GRANT EXECUTE ON FUNCTION stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text) TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.claim_account_setup_notification() TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.complete_account_setup_notification(uuid,uuid,text) TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer) TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.account_setup_notification_counts() TO stonez_bulk_worker_processing;

-- Fail closed if a pre-existing private table or function has an unexpected
-- owner or if any direct table privilege has leaked to the processing login.
DO $outbox_boundary_check$
DECLARE
    v_owner oid := (SELECT oid FROM pg_roles WHERE rolname='stonez_bulk_context_owner');
    v_table oid := to_regclass('stonez_context_private.account_setup_outbox');
    v_function regprocedure;
BEGIN
    IF v_table IS NULL OR (SELECT relowner FROM pg_class WHERE oid=v_table) <> v_owner THEN
        RAISE EXCEPTION 'account-setup outbox must be owned by stonez_bulk_context_owner';
    END IF;
    IF has_table_privilege('stonez_bulk_worker_processing',v_table,'SELECT')
       OR has_table_privilege('stonez_bulk_worker_processing',v_table,'INSERT')
       OR has_table_privilege('stonez_bulk_worker_processing',v_table,'UPDATE')
       OR has_table_privilege('stonez_bulk_worker_processing',v_table,'DELETE')
       OR has_table_privilege('stonez_bulk_context_key_admin',v_table,'SELECT')
       OR has_table_privilege('stonez_bulk_context_key_admin',v_table,'INSERT')
       OR has_table_privilege('stonez_bulk_context_key_admin',v_table,'UPDATE')
       OR has_table_privilege('stonez_bulk_context_key_admin',v_table,'DELETE') THEN
        RAISE EXCEPTION 'outbox table privileges leaked to processing or key-admin role';
    END IF;
    FOREACH v_function IN ARRAY ARRAY[
        'stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text)'::regprocedure,
        'stonez_context_private.claim_account_setup_notification()'::regprocedure,
        'stonez_context_private.complete_account_setup_notification(uuid,uuid,text)'::regprocedure,
        'stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer)'::regprocedure,
        'stonez_context_private.account_setup_notification_counts()'::regprocedure
    ] LOOP
        IF NOT EXISTS (
            SELECT 1 FROM pg_proc
             WHERE oid=v_function AND prosecdef AND proowner=v_owner
               AND 'search_path=pg_catalog' = ANY(COALESCE(proconfig, ARRAY[]::text[]))
        ) THEN
            RAISE EXCEPTION 'outbox function % must be SECURITY DEFINER, context-owner-owned, and use fixed search_path',v_function;
        END IF;
    END LOOP;
END
$outbox_boundary_check$;

RESET ROLE;
COMMIT;
