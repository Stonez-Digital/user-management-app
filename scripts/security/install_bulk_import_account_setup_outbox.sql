-- Install the private, signed-context-bound account-setup notification outbox.
-- Run only after install_bulk_import_context_verifier.sql and
-- install_bulk_import_context_worker_policies.sql have been reviewed and installed.
-- This is an operator-run script; it is not an application migration.
BEGIN;

DO $preflight$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_context_owner' AND NOT rolcanlogin AND NOT rolinherit)
       OR NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_worker_processing' AND NOT rolcanlogin)
       OR NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'stonez_context_private'
                      AND nspowner = (SELECT oid FROM pg_roles WHERE rolname = 'stonez_bulk_context_owner')) THEN
        RAISE EXCEPTION 'signed-context owner, processing group, and private schema must be installed first';
    END IF;
    IF NOT has_schema_privilege('stonez_bulk_worker_processing','stonez_context_private','USAGE') THEN
        RAISE EXCEPTION 'processing group must have private schema USAGE from verifier installation';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE oid = to_regclass('public.bulk_import_jobs')) THEN
        RAISE EXCEPTION 'public.bulk_import_jobs is required';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE oid = to_regclass('public.users')) THEN
        RAISE EXCEPTION 'public.users is required';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_class AS relation
         WHERE relation.oid = 'public.users'::regclass
           AND relation.relowner = (SELECT oid FROM pg_roles WHERE rolname = current_user)
    ) THEN
        RAISE EXCEPTION 'current role % must own public.users to install the restricted identity-read policy', current_user;
    END IF;
END
$preflight$;

GRANT stonez_bulk_context_owner TO CURRENT_USER WITH INHERIT FALSE, SET TRUE;
-- Install the narrow identity-read boundary while running as the application
-- table owner; CREATE POLICY and GRANT require that ownership boundary.
GRANT USAGE ON SCHEMA public TO stonez_bulk_context_owner;
GRANT SELECT (id,school_id,email,name,role) ON public.users TO stonez_bulk_context_owner;
DROP POLICY IF EXISTS bulk_worker_outbox_owner_user_read ON public.users;
CREATE POLICY bulk_worker_outbox_owner_user_read ON public.users
    FOR SELECT TO stonez_bulk_context_owner USING (true);

SET LOCAL ROLE stonez_bulk_context_owner;

CREATE TABLE IF NOT EXISTS stonez_context_private.account_setup_outbox (
    event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL,
    job_id uuid NOT NULL,
    user_id uuid NOT NULL,
    recipient_email text NOT NULL CHECK (recipient_email <> '' AND length(recipient_email) <= 320),
    recipient_name text NOT NULL DEFAULT '' CHECK (length(recipient_name) <= 200),
    account_role text NOT NULL CHECK (account_role IN ('student','teacher','parent')),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','delivering','retry','delivered','failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0 AND attempts <= 8),
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    lease_id uuid,
    lease_expires_at timestamptz,
    provider_message_id text,
    last_error text NOT NULL DEFAULT '' CHECK (length(last_error) <= 1000),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    delivered_at timestamptz,
    CONSTRAINT account_setup_outbox_job_user_unique UNIQUE (job_id,user_id),
    CONSTRAINT account_setup_outbox_lease_pair CHECK (
        (lease_id IS NULL AND lease_expires_at IS NULL)
        OR (lease_id IS NOT NULL AND lease_expires_at IS NOT NULL)
    ),
    CONSTRAINT account_setup_outbox_delivery_state CHECK (
        (status = 'delivered' AND delivered_at IS NOT NULL AND lease_id IS NULL)
        OR (status <> 'delivered' AND delivered_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS account_setup_outbox_due_idx
    ON stonez_context_private.account_setup_outbox (school_id,job_id,next_attempt_at,created_at)
    WHERE status IN ('pending','retry','delivering');

REVOKE ALL ON stonez_context_private.account_setup_outbox
    FROM PUBLIC, stonez_bulk_worker_processing, stonez_bulk_context_key_admin;
GRANT SELECT, INSERT, UPDATE ON stonez_context_private.account_setup_outbox
    TO stonez_bulk_context_owner;

CREATE OR REPLACE FUNCTION stonez_context_private.enqueue_account_setup_notice(p_user_id uuid)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
    v_email text;
    v_name text;
    v_role text;
    v_send boolean;
    v_event uuid;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    IF v_school IS NULL OR v_job IS NULL OR p_user_id IS NULL THEN
        RAISE EXCEPTION 'verified worker context and user ID are required' USING ERRCODE = '42501';
    END IF;
    SELECT send_credentials INTO v_send
      FROM public.bulk_import_jobs
     WHERE id = v_job AND school_id = v_school
       AND initiated_by = stonez_context_private.current_worker_initiator_id();
    IF NOT FOUND OR NOT v_send THEN
        RAISE EXCEPTION 'verified import job is not authorized for account setup notices' USING ERRCODE = '42501';
    END IF;
    SELECT email,name,role INTO v_email,v_name,v_role
      FROM public.users
     WHERE id = p_user_id AND school_id = v_school;
    IF NOT FOUND OR v_email IS NULL OR btrim(v_email) = ''
       OR v_role NOT IN ('student','teacher','parent') THEN
        RAISE EXCEPTION 'user does not exist in the verified school or has an ineligible role' USING ERRCODE = '22023';
    END IF;

    INSERT INTO stonez_context_private.account_setup_outbox
        (school_id,job_id,user_id,recipient_email,recipient_name,account_role)
    VALUES (v_school,v_job,p_user_id,v_email,COALESCE(v_name,''),v_role)
    ON CONFLICT (job_id,user_id) DO NOTHING
    RETURNING event_id INTO v_event;
    IF v_event IS NULL THEN
        SELECT event_id INTO v_event
          FROM stonez_context_private.account_setup_outbox
         WHERE job_id = v_job AND user_id = p_user_id AND school_id = v_school;
    END IF;
    RETURN v_event;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.claim_account_setup_notice()
RETURNS TABLE (
    event_id uuid,
    user_id uuid,
    recipient_email text,
    recipient_name text,
    account_role text,
    attempts integer,
    idempotency_key text,
    lease_id uuid
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
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
        RAISE EXCEPTION 'verified worker context is required' USING ERRCODE = '42501';
    END IF;
    UPDATE stonez_context_private.account_setup_outbox AS expired
       SET status = 'failed', lease_id = NULL, lease_expires_at = NULL,
           last_error = 'delivery lease expired after maximum attempts', updated_at = clock_timestamp()
     WHERE expired.school_id = v_school AND expired.job_id = v_job
       AND expired.status = 'delivering' AND expired.lease_expires_at <= clock_timestamp()
       AND expired.attempts >= 8;
    SELECT outbox.event_id INTO v_event
      FROM stonez_context_private.account_setup_outbox AS outbox
     WHERE outbox.school_id = v_school AND outbox.job_id = v_job
       AND outbox.attempts < 8
       AND (
           (outbox.status IN ('pending','retry') AND outbox.next_attempt_at <= clock_timestamp())
           OR (outbox.status = 'delivering' AND outbox.lease_expires_at <= clock_timestamp())
       )
     ORDER BY outbox.created_at,outbox.event_id
     LIMIT 1
     FOR UPDATE SKIP LOCKED;
    IF v_event IS NULL THEN
        RETURN;
    END IF;
    v_lease := gen_random_uuid();
    RETURN QUERY
    UPDATE stonez_context_private.account_setup_outbox AS outbox
       SET status = 'delivering',
           attempts = outbox.attempts + 1,
           lease_id = v_lease,
           lease_expires_at = clock_timestamp() + interval '2 minutes',
           updated_at = clock_timestamp(),
           last_error = ''
     WHERE outbox.event_id = v_event
    RETURNING outbox.event_id,outbox.user_id,outbox.recipient_email,
              outbox.recipient_name,outbox.account_role,outbox.attempts,
              outbox.event_id::text,outbox.lease_id;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.complete_account_setup_notice(
    p_event_id uuid,p_lease_id uuid,p_provider_message_id text DEFAULT NULL
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    IF v_school IS NULL OR v_job IS NULL OR p_event_id IS NULL OR p_lease_id IS NULL THEN
        RAISE EXCEPTION 'verified worker context, event ID, and lease ID are required' USING ERRCODE = '42501';
    END IF;
    UPDATE stonez_context_private.account_setup_outbox
       SET status = 'delivered', delivered_at = clock_timestamp(),
           provider_message_id = left(COALESCE(p_provider_message_id,''),200),
           lease_id = NULL, lease_expires_at = NULL,
           last_error = '', updated_at = clock_timestamp()
     WHERE event_id = p_event_id AND school_id = v_school AND job_id = v_job
       AND status = 'delivering' AND lease_id = p_lease_id
       AND lease_expires_at > clock_timestamp();
    RETURN FOUND;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.fail_account_setup_notice(
    p_event_id uuid,p_lease_id uuid,p_error text
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_school uuid;
    v_job uuid;
    v_attempts integer;
    v_next timestamptz;
BEGIN
    v_school := stonez_context_private.current_worker_school_id();
    v_job := stonez_context_private.current_worker_job_id();
    IF v_school IS NULL OR v_job IS NULL OR p_event_id IS NULL OR p_lease_id IS NULL THEN
        RAISE EXCEPTION 'verified worker context, event ID, and lease ID are required' USING ERRCODE = '42501';
    END IF;
    SELECT attempts INTO v_attempts
      FROM stonez_context_private.account_setup_outbox
     WHERE event_id = p_event_id AND school_id = v_school AND job_id = v_job
       AND status = 'delivering' AND lease_id = p_lease_id
       AND lease_expires_at > clock_timestamp()
     FOR UPDATE;
    IF NOT FOUND THEN
        RETURN false;
    END IF;
    IF v_attempts >= 8 THEN
        UPDATE stonez_context_private.account_setup_outbox
           SET status = 'failed', lease_id = NULL, lease_expires_at = NULL,
               last_error = left(COALESCE(p_error,'delivery failed'),1000),
               updated_at = clock_timestamp()
         WHERE event_id = p_event_id;
    ELSE
        v_next := clock_timestamp() + make_interval(secs => least(3600,15 * power(2::numeric,(v_attempts - 1)::numeric)::integer));
        UPDATE stonez_context_private.account_setup_outbox
           SET status = 'retry', next_attempt_at = v_next,
               lease_id = NULL, lease_expires_at = NULL,
               last_error = left(COALESCE(p_error,'delivery failed'),1000),
               updated_at = clock_timestamp()
         WHERE event_id = p_event_id;
    END IF;
    RETURN true;
END
$fn$;

REVOKE ALL ON FUNCTION stonez_context_private.enqueue_account_setup_notice(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.claim_account_setup_notice() FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.complete_account_setup_notice(uuid,uuid,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.fail_account_setup_notice(uuid,uuid,text) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION stonez_context_private.enqueue_account_setup_notice(uuid)
    TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.claim_account_setup_notice()
    TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.complete_account_setup_notice(uuid,uuid,text)
    TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.fail_account_setup_notice(uuid,uuid,text)
    TO stonez_bulk_worker_processing;

RESET ROLE;
COMMIT;
