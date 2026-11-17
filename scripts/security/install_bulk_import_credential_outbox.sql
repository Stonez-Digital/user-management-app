-- Private durable account-setup notification outbox for signed bulk imports.
-- Install only after the signed-context verifier and worker RLS policies.
-- This script is operator-run and is not an application migration.
BEGIN;

DO $preflight$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='stonez_bulk_context_owner' AND NOT rolcanlogin)
       OR NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='stonez_bulk_worker_processing' AND NOT rolcanlogin)
       OR to_regprocedure('stonez_context_private.is_eligible_processing_login()') IS NULL
       OR to_regprocedure('stonez_context_private.current_worker_school_id()') IS NULL
       OR to_regprocedure('stonez_context_private.current_worker_job_id()') IS NULL THEN
        RAISE EXCEPTION 'signed-context verifier roles and accessors must be installed before the credential outbox';
    END IF;
    IF NOT has_schema_privilege('stonez_bulk_context_owner','stonez_context_private','CREATE') THEN
        RAISE EXCEPTION 'stonez_bulk_context_owner must own/create in stonez_context_private';
    END IF;
END
$preflight$;

SET LOCAL ROLE stonez_bulk_context_owner;

CREATE TABLE IF NOT EXISTS stonez_context_private.account_setup_outbox (
    event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL,
    job_id uuid NOT NULL,
    user_id uuid NOT NULL,
    recipient_email text NOT NULL CHECK (length(recipient_email) BETWEEN 3 AND 320 AND recipient_email !~ '[[:space:][:cntrl:]]'),
    display_name text NOT NULL DEFAULT '' CHECK (length(display_name) <= 200),
    account_role text NOT NULL CHECK (account_role IN ('student','teacher','parent')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed','delivered','failed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    lease_id uuid,
    lease_expires_at timestamptz,
    provider_message_id text CHECK (provider_message_id IS NULL OR length(provider_message_id) <= 256),
    last_error text NOT NULL DEFAULT '' CHECK (length(last_error) <= 500),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    delivered_at timestamptz,
    CONSTRAINT account_setup_outbox_job_user_unique UNIQUE(job_id,user_id),
    CONSTRAINT account_setup_outbox_lease_consistency CHECK (
        (status = 'claimed' AND lease_id IS NOT NULL AND lease_expires_at IS NOT NULL)
        OR (status <> 'claimed' AND lease_id IS NULL AND lease_expires_at IS NULL)
    ),
    CONSTRAINT account_setup_outbox_delivery_consistency CHECK (
        (status = 'delivered' AND delivered_at IS NOT NULL)
        OR (status <> 'delivered' AND delivered_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS account_setup_outbox_due_idx
    ON stonez_context_private.account_setup_outbox(next_attempt_at,created_at)
    WHERE status='pending';
CREATE INDEX IF NOT EXISTS account_setup_outbox_expired_lease_idx
    ON stonez_context_private.account_setup_outbox(lease_expires_at)
    WHERE status='claimed';

REVOKE ALL ON stonez_context_private.account_setup_outbox
    FROM PUBLIC, stonez_bulk_worker_processing, stonez_bulk_context_key_admin;

CREATE OR REPLACE FUNCTION stonez_context_private.enqueue_account_setup_notification(
    p_user_id uuid, p_recipient_email text, p_display_name text, p_account_role text
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_school_id uuid;
    v_job_id uuid;
    v_event_id uuid;
BEGIN
    IF NOT stonez_context_private.is_eligible_processing_login() THEN
        RAISE EXCEPTION 'caller is not an eligible signed bulk worker' USING ERRCODE='42501';
    END IF;
    v_school_id := stonez_context_private.current_worker_school_id();
    v_job_id := stonez_context_private.current_worker_job_id();
    IF v_school_id IS NULL OR v_job_id IS NULL THEN
        RAISE EXCEPTION 'verified worker transaction context is required' USING ERRCODE='28000';
    END IF;
    IF p_user_id IS NULL OR p_recipient_email IS NULL
       OR length(p_recipient_email) NOT BETWEEN 3 AND 320
       OR p_recipient_email ~ '[[:space:][:cntrl:]]'
       OR p_display_name IS NULL OR length(p_display_name) > 200
       OR p_account_role NOT IN ('student','teacher','parent') THEN
        RAISE EXCEPTION 'invalid account-setup notification fields' USING ERRCODE='22023';
    END IF;

    INSERT INTO stonez_context_private.account_setup_outbox(
        school_id,job_id,user_id,recipient_email,display_name,account_role
    ) VALUES (v_school_id,v_job_id,p_user_id,lower(p_recipient_email),p_display_name,p_account_role)
    ON CONFLICT (job_id,user_id) DO NOTHING
    RETURNING event_id INTO v_event_id;

    IF v_event_id IS NULL THEN
        SELECT event_id INTO v_event_id
          FROM stonez_context_private.account_setup_outbox
         WHERE school_id=v_school_id AND job_id=v_job_id AND user_id=p_user_id;
        IF v_event_id IS NULL THEN
            RAISE EXCEPTION 'existing notification belongs to a different tenant or job' USING ERRCODE='42501';
        END IF;
    END IF;
    RETURN v_event_id;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.claim_account_setup_notification()
RETURNS TABLE(
    event_id uuid, user_id uuid, recipient_email text, display_name text,
    account_role text, delivery_lease uuid, attempt_count integer
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_school_id uuid;
    v_job_id uuid;
    v_event stonez_context_private.account_setup_outbox%ROWTYPE;
BEGIN
    IF NOT stonez_context_private.is_eligible_processing_login() THEN
        RAISE EXCEPTION 'caller is not an eligible signed bulk worker' USING ERRCODE='42501';
    END IF;
    v_school_id := stonez_context_private.current_worker_school_id();
    v_job_id := stonez_context_private.current_worker_job_id();
    IF v_school_id IS NULL OR v_job_id IS NULL THEN
        RAISE EXCEPTION 'verified worker transaction context is required' USING ERRCODE='28000';
    END IF;

    UPDATE stonez_context_private.account_setup_outbox AS outbox
       SET status='failed',lease_id=NULL,lease_expires_at=NULL,
           last_error='delivery lease expired after maximum attempts',updated_at=clock_timestamp()
     WHERE outbox.school_id=v_school_id AND outbox.job_id=v_job_id
       AND outbox.status='claimed' AND outbox.lease_expires_at <= clock_timestamp()
       AND outbox.attempt_count >= 12;

    SELECT outbox.* INTO v_event
      FROM stonez_context_private.account_setup_outbox AS outbox
     WHERE outbox.school_id=v_school_id AND outbox.job_id=v_job_id
       AND outbox.attempt_count < 12
       AND ((outbox.status='pending' AND outbox.next_attempt_at <= clock_timestamp())
         OR (outbox.status='claimed' AND outbox.lease_expires_at <= clock_timestamp()))
     ORDER BY outbox.created_at,outbox.event_id
     FOR UPDATE SKIP LOCKED
     LIMIT 1;
    IF NOT FOUND THEN RETURN; END IF;

    UPDATE stonez_context_private.account_setup_outbox AS outbox
       SET status='claimed', attempt_count=outbox.attempt_count+1,
           lease_id=gen_random_uuid(), lease_expires_at=clock_timestamp()+interval '2 minutes',
           last_error='', updated_at=clock_timestamp()
     WHERE outbox.event_id=v_event.event_id
     RETURNING outbox.* INTO v_event;

    RETURN QUERY SELECT v_event.event_id,v_event.user_id,v_event.recipient_email,
        v_event.display_name,v_event.account_role,v_event.lease_id,v_event.attempt_count;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.complete_account_setup_notification(
    p_event_id uuid, p_delivery_lease uuid, p_provider_message_id text DEFAULT NULL
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_rows integer;
BEGIN
    IF NOT stonez_context_private.is_eligible_processing_login() THEN
        RAISE EXCEPTION 'caller is not an eligible signed bulk worker' USING ERRCODE='42501';
    END IF;
    UPDATE stonez_context_private.account_setup_outbox AS outbox
       SET status='delivered',lease_id=NULL,lease_expires_at=NULL,
           provider_message_id=left(p_provider_message_id,256),
           delivered_at=clock_timestamp(),updated_at=clock_timestamp(),last_error=''
     WHERE outbox.event_id=p_event_id
       AND outbox.school_id=stonez_context_private.current_worker_school_id()
       AND outbox.job_id=stonez_context_private.current_worker_job_id()
       AND outbox.status='claimed' AND outbox.lease_id=p_delivery_lease
       AND outbox.lease_expires_at > clock_timestamp();
    GET DIAGNOSTICS v_rows=ROW_COUNT;
    RETURN v_rows=1;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.retry_account_setup_notification(
    p_event_id uuid, p_delivery_lease uuid, p_error text, p_delay_seconds integer
)
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_status text;
BEGIN
    IF NOT stonez_context_private.is_eligible_processing_login() THEN
        RAISE EXCEPTION 'caller is not an eligible signed bulk worker' USING ERRCODE='42501';
    END IF;
    IF p_delay_seconds IS NULL OR p_delay_seconds NOT BETWEEN 1 AND 3600 THEN
        RAISE EXCEPTION 'retry delay must be between 1 and 3600 seconds' USING ERRCODE='22023';
    END IF;
    UPDATE stonez_context_private.account_setup_outbox AS outbox
       SET status=CASE WHEN outbox.attempt_count >= 12 THEN 'failed' ELSE 'pending' END,
           next_attempt_at=clock_timestamp()+make_interval(secs=>p_delay_seconds),
           lease_id=NULL,lease_expires_at=NULL,
           last_error=left(coalesce(p_error,'delivery failed'),500),
           updated_at=clock_timestamp()
     WHERE outbox.event_id=p_event_id
       AND outbox.school_id=stonez_context_private.current_worker_school_id()
       AND outbox.job_id=stonez_context_private.current_worker_job_id()
       AND outbox.status='claimed' AND outbox.lease_id=p_delivery_lease
       AND outbox.lease_expires_at > clock_timestamp()
    RETURNING outbox.status INTO v_status;
    RETURN v_status;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.account_setup_notification_counts()
RETURNS TABLE(delivered_count integer, failed_count integer, pending_count integer)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    IF NOT stonez_context_private.is_eligible_processing_login() THEN
        RAISE EXCEPTION 'caller is not an eligible signed bulk worker' USING ERRCODE='42501';
    END IF;
    RETURN QUERY
    SELECT
        count(*) FILTER (WHERE outbox.status='delivered')::integer,
        count(*) FILTER (WHERE outbox.status='failed')::integer,
        count(*) FILTER (WHERE outbox.status IN ('pending','claimed'))::integer
      FROM stonez_context_private.account_setup_outbox AS outbox
     WHERE outbox.school_id=stonez_context_private.current_worker_school_id()
       AND outbox.job_id=stonez_context_private.current_worker_job_id();
END
$fn$;

REVOKE ALL ON FUNCTION stonez_context_private.account_setup_notification_counts() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_context_private.account_setup_notification_counts() TO stonez_bulk_worker_processing;

REVOKE ALL ON FUNCTION stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.claim_account_setup_notification() FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.complete_account_setup_notification(uuid,uuid,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text) TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.claim_account_setup_notification() TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.complete_account_setup_notification(uuid,uuid,text) TO stonez_bulk_worker_processing;
GRANT EXECUTE ON FUNCTION stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer) TO stonez_bulk_worker_processing;

RESET ROLE;
COMMIT;
