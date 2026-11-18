-- School subscription foundation.
-- REVIEW ONLY: intentionally not registered in the automatic application
-- migration runner. Do not apply to production until reviewed and approved.

BEGIN;

CREATE TABLE IF NOT EXISTS billing_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code varchar(30) NOT NULL UNIQUE,
    name varchar(80) NOT NULL,
    amount_minor bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    currency varchar(3) NOT NULL DEFAULT 'NGN',
    billing_interval varchar(20) NOT NULL DEFAULT 'term',
    max_students integer,
    max_teachers integer,
    storage_limit_bytes bigint,
    active boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz,
    updated_at timestamptz,
    CONSTRAINT billing_plans_code_check CHECK (code IN ('trial','basic','professional','premium','enterprise')),
    CONSTRAINT billing_plans_limits_check CHECK (
        (max_students IS NULL OR max_students >= 0) AND
        (max_teachers IS NULL OR max_teachers >= 0) AND
        (storage_limit_bytes IS NULL OR storage_limit_bytes >= 0)
    )
);

INSERT INTO billing_plans
    (code, name, amount_minor, currency, billing_interval, max_students, max_teachers, storage_limit_bytes, active, sort_order)
VALUES
    ('trial', 'Free Trial', 0, 'NGN', 'term', 50, 5, 1073741824, true, 0),
    ('basic', 'Basic', 6500000, 'NGN', 'term', 200, 15, 5368709120, true, 1),
    ('professional', 'Professional', 10000000, 'NGN', 'term', 500, 40, 21474836480, true, 2),
    ('premium', 'Premium', 15000000, 'NGN', 'term', 1500, 100, 53687091200, true, 3),
    ('enterprise', 'Enterprise', 0, 'NGN', 'term', NULL, NULL, NULL, true, 4)
ON CONFLICT (code) DO NOTHING;

-- Composite keys let PostgreSQL enforce that a subscription term belongs to its school.
CREATE UNIQUE INDEX IF NOT EXISTS uq_terms_school_id_id ON terms(school_id, id);

CREATE TABLE IF NOT EXISTS school_subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    plan_id uuid NOT NULL REFERENCES billing_plans(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    term_id uuid NOT NULL,
    status varchar(24) NOT NULL,
    is_trial boolean NOT NULL DEFAULT false,
    amount_minor bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    currency varchar(3) NOT NULL DEFAULT 'NGN',
    starts_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    cancelled_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    CONSTRAINT school_subscriptions_status_check CHECK (status IN ('active','expired','cancelled','pending_payment')),
    CONSTRAINT school_subscriptions_dates_check CHECK (expires_at > starts_at),
    CONSTRAINT school_subscriptions_trial_amount_check CHECK (NOT is_trial OR amount_minor = 0),
    CONSTRAINT school_subscriptions_school_term_fk FOREIGN KEY (school_id, term_id)
        REFERENCES terms(school_id, id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_school_subscriptions_school_id_id
    ON school_subscriptions(school_id, id);
CREATE INDEX IF NOT EXISTS idx_school_subscriptions_school_status
    ON school_subscriptions(school_id, status, expires_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_school_subscriptions_one_active
    ON school_subscriptions(school_id) WHERE status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS uq_school_subscriptions_one_trial
    ON school_subscriptions(school_id) WHERE is_trial = true;

CREATE TABLE IF NOT EXISTS subscription_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    subscription_id uuid NOT NULL,
    actor_id uuid REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    event_type varchar(50) NOT NULL,
    from_status varchar(24),
    to_status varchar(24),
    metadata text,
    created_at timestamptz,
    CONSTRAINT subscription_events_school_subscription_fk FOREIGN KEY (school_id, subscription_id)
        REFERENCES school_subscriptions(school_id, id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_subscription_events_school_created
    ON subscription_events(school_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subscription_events_subscription
    ON subscription_events(subscription_id, created_at DESC);

ALTER TABLE billing_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE school_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_events ENABLE ROW LEVEL SECURITY;

REVOKE ALL ON TABLE billing_plans FROM PUBLIC;
REVOKE ALL ON TABLE school_subscriptions FROM PUBLIC;
REVOKE ALL ON TABLE subscription_events FROM PUBLIC;

DO $$
DECLARE role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('REVOKE ALL ON TABLE billing_plans FROM %I', role_name);
            EXECUTE format('REVOKE ALL ON TABLE school_subscriptions FROM %I', role_name);
            EXECUTE format('REVOKE ALL ON TABLE subscription_events FROM %I', role_name);
        END IF;
    END LOOP;
END $$;

COMMIT;
