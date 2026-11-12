-- School subscription checkout persistence.
-- REVIEW ONLY: apply through the approved migration workflow after review.
-- This migration is intentionally not registered in the automatic GORM runner.

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS uq_school_subscriptions_one_pending
    ON public.school_subscriptions(school_id)
    WHERE status = 'pending_payment';

CREATE TABLE IF NOT EXISTS public.subscription_payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES public.schools(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    subscription_id uuid NOT NULL,
    provider varchar(30) NOT NULL,
    reference varchar(100) NOT NULL UNIQUE,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency varchar(3) NOT NULL DEFAULT 'NGN' CHECK (currency = 'NGN'),
    status varchar(24) NOT NULL CHECK (status IN ('pending','succeeded','failed')),
    authorization_url varchar(1000),
    access_code varchar(255),
    paid_at timestamptz,
    metadata text,
    created_at timestamptz,
    updated_at timestamptz,
    CONSTRAINT subscription_payments_school_subscription_fk
        FOREIGN KEY (school_id, subscription_id)
        REFERENCES public.school_subscriptions(school_id, id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_subscription_payments_school_created
    ON public.subscription_payments(school_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subscription_payments_subscription
    ON public.subscription_payments(subscription_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subscription_payments_school_subscription
    ON public.subscription_payments(school_id, subscription_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subscription_payments_status
    ON public.subscription_payments(status, created_at);

ALTER TABLE public.subscription_payments ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON TABLE public.subscription_payments FROM PUBLIC;

DO $$
DECLARE role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('REVOKE ALL ON TABLE public.subscription_payments FROM %I', role_name);
        END IF;
    END LOOP;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'service_role') THEN
        GRANT ALL ON TABLE public.billing_plans TO service_role;
        GRANT ALL ON TABLE public.school_subscriptions TO service_role;
        GRANT ALL ON TABLE public.subscription_events TO service_role;
        GRANT ALL ON TABLE public.subscription_payments TO service_role;
    END IF;
END $;

COMMIT;
