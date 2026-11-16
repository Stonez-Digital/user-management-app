-- Operator-run billing RLS and least-privilege grants.
-- Run only after both subscription schema migrations and role provisioning.
-- This is deliberately not part of application startup or automatic migrations.
BEGIN;

DO $$
DECLARE api_role pg_roles%ROWTYPE;
BEGIN
    SELECT * INTO api_role FROM pg_roles WHERE rolname = 'stonez_api_runtime';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'stonez_api_runtime must be provisioned before billing RLS';
    END IF;
    IF api_role.rolcanlogin OR api_role.rolsuper OR api_role.rolbypassrls
       OR api_role.rolcreatedb OR api_role.rolcreaterole OR NOT api_role.rolinherit THEN
        RAISE EXCEPTION 'stonez_api_runtime has unsafe role attributes';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles AS privileged
        WHERE (privileged.rolsuper OR privileged.rolbypassrls)
          AND pg_has_role('stonez_api_runtime', privileged.oid, 'MEMBER')
    ) THEN
        RAISE EXCEPTION 'stonez_api_runtime inherits a privileged role';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_class
        WHERE oid IN (
            'public.billing_plans'::regclass,
            'public.school_subscriptions'::regclass,
            'public.subscription_events'::regclass,
            'public.subscription_payments'::regclass
        )
        AND relowner = api_role.oid
    ) THEN
        RAISE EXCEPTION 'stonez_api_runtime must not own billing tables';
    END IF;
END $$;

-- The plan catalogue is global but read-only to the application runtime.
ALTER TABLE public.billing_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.billing_plans FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS billing_plans_runtime_active_read ON public.billing_plans;
CREATE POLICY billing_plans_runtime_active_read
    ON public.billing_plans FOR SELECT TO stonez_api_runtime
    USING (active = true);

-- All remaining billing records are school-owned. Missing tenant context
-- evaluates to NULL and therefore denies both reads and writes.
ALTER TABLE public.school_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.school_subscriptions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS school_subscriptions_runtime_select ON public.school_subscriptions;
CREATE POLICY school_subscriptions_runtime_select
    ON public.school_subscriptions FOR SELECT TO stonez_api_runtime
    USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);
DROP POLICY IF EXISTS school_subscriptions_runtime_insert ON public.school_subscriptions;
CREATE POLICY school_subscriptions_runtime_insert
    ON public.school_subscriptions FOR INSERT TO stonez_api_runtime
    WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);
DROP POLICY IF EXISTS school_subscriptions_runtime_update ON public.school_subscriptions;
CREATE POLICY school_subscriptions_runtime_update
    ON public.school_subscriptions FOR UPDATE TO stonez_api_runtime
    USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid)
    WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);

ALTER TABLE public.subscription_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.subscription_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS subscription_events_runtime_select ON public.subscription_events;
CREATE POLICY subscription_events_runtime_select
    ON public.subscription_events FOR SELECT TO stonez_api_runtime
    USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);
DROP POLICY IF EXISTS subscription_events_runtime_insert ON public.subscription_events;
CREATE POLICY subscription_events_runtime_insert
    ON public.subscription_events FOR INSERT TO stonez_api_runtime
    WITH CHECK (
        school_id = nullif(current_setting('app.current_school_id', true), '')::uuid
        AND (actor_id IS NULL OR actor_id = nullif(current_setting('app.current_user_id', true), '')::uuid)
    );

ALTER TABLE public.subscription_payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.subscription_payments FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS subscription_payments_runtime_select ON public.subscription_payments;
CREATE POLICY subscription_payments_runtime_select
    ON public.subscription_payments FOR SELECT TO stonez_api_runtime
    USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);
DROP POLICY IF EXISTS subscription_payments_runtime_insert ON public.subscription_payments;
CREATE POLICY subscription_payments_runtime_insert
    ON public.subscription_payments FOR INSERT TO stonez_api_runtime
    WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);
DROP POLICY IF EXISTS subscription_payments_runtime_update ON public.subscription_payments;
CREATE POLICY subscription_payments_runtime_update
    ON public.subscription_payments FOR UPDATE TO stonez_api_runtime
    USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid)
    WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid);

REVOKE ALL ON TABLE public.billing_plans, public.school_subscriptions,
    public.subscription_events, public.subscription_payments FROM PUBLIC;
REVOKE ALL ON TABLE public.billing_plans, public.school_subscriptions,
    public.subscription_events, public.subscription_payments FROM stonez_api_runtime;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON TABLE public.billing_plans, public.school_subscriptions,
            public.subscription_events, public.subscription_payments FROM anon;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON TABLE public.billing_plans, public.school_subscriptions,
            public.subscription_events, public.subscription_payments FROM authenticated;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'service_role') THEN
        REVOKE ALL ON TABLE public.billing_plans, public.school_subscriptions,
            public.subscription_events, public.subscription_payments FROM service_role;
    END IF;
END $$;

GRANT SELECT ON TABLE public.billing_plans TO stonez_api_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.school_subscriptions TO stonez_api_runtime;
GRANT SELECT, INSERT ON TABLE public.subscription_events TO stonez_api_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.subscription_payments TO stonez_api_runtime;

-- Assert the exact intended policies and grants before committing.
DO $$
DECLARE expected integer;
BEGIN
    SELECT COUNT(*) INTO expected
    FROM pg_policies
    WHERE schemaname = 'public'
      AND (
        (tablename = 'billing_plans' AND policyname = 'billing_plans_runtime_active_read')
        OR (tablename = 'school_subscriptions' AND policyname IN ('school_subscriptions_runtime_select','school_subscriptions_runtime_insert','school_subscriptions_runtime_update'))
        OR (tablename = 'subscription_events' AND policyname IN ('subscription_events_runtime_select','subscription_events_runtime_insert'))
        OR (tablename = 'subscription_payments' AND policyname IN ('subscription_payments_runtime_select','subscription_payments_runtime_insert','subscription_payments_runtime_update'))
      );
    IF expected <> 9 THEN
        RAISE EXCEPTION 'expected 9 billing RLS policies, found %', expected;
    END IF;
    IF has_table_privilege('stonez_api_runtime','public.billing_plans','INSERT')
       OR has_table_privilege('stonez_api_runtime','public.billing_plans','UPDATE')
       OR has_table_privilege('stonez_api_runtime','public.billing_plans','DELETE')
       OR has_table_privilege('stonez_api_runtime','public.subscription_events','UPDATE')
       OR has_table_privilege('stonez_api_runtime','public.subscription_events','DELETE')
       OR has_table_privilege('stonez_api_runtime','public.subscription_payments','DELETE')
       OR has_table_privilege('stonez_api_runtime','public.school_subscriptions','DELETE') THEN
        RAISE EXCEPTION 'billing runtime role has unexpected write privileges';
    END IF;
END $$;

COMMIT;
