-- REVIEW-ONLY foundation for the signed bulk-import context verifier.
-- This operator-run script is intentionally not called by application startup.
-- Do not run against staging or production until the verifier and role tests pass.
BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_worker') THEN
        CREATE ROLE stonez_bulk_worker NOLOGIN NOSUPERUSER NOBYPASSRLS INHERIT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_context_owner') THEN
        CREATE ROLE stonez_bulk_context_owner NOLOGIN NOSUPERUSER NOBYPASSRLS NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_context_key_admin') THEN
        CREATE ROLE stonez_bulk_context_key_admin NOLOGIN NOSUPERUSER NOBYPASSRLS NOINHERIT;
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname IN ('stonez_bulk_worker','stonez_bulk_context_owner','stonez_bulk_context_key_admin')
          AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole)
    ) THEN
        RAISE EXCEPTION 'bulk context roles have unsafe role attributes';
    END IF;
END
$$;

CREATE SCHEMA IF NOT EXISTS stonez_context_private AUTHORIZATION stonez_bulk_context_owner;
REVOKE ALL ON SCHEMA stonez_context_private FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO stonez_bulk_context_owner;
GRANT USAGE ON SCHEMA stonez_context_private TO stonez_bulk_worker, stonez_bulk_context_key_admin;
GRANT CREATE ON SCHEMA stonez_context_private TO stonez_bulk_context_owner;

CREATE TABLE IF NOT EXISTS stonez_context_private.verification_keys (
    key_id text PRIMARY KEY CHECK (key_id <> '' AND length(key_id) <= 64),
    key_bytes bytea NOT NULL CHECK (octet_length(key_bytes) >= 32),
    not_before timestamptz NOT NULL,
    not_after timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (not_after IS NULL OR not_after > not_before)
);

CREATE TABLE IF NOT EXISTS stonez_context_private.verified_contexts (
    backend_pid integer NOT NULL,
    transaction_id xid8 NOT NULL,
    assertion_id uuid NOT NULL,
    job_id uuid NOT NULL,
    school_id uuid NOT NULL,
    initiated_by uuid NOT NULL,
    worker_id uuid NOT NULL,
    lease_id uuid NOT NULL,
    verified_at timestamptz NOT NULL,
    PRIMARY KEY (backend_pid, transaction_id),
    CONSTRAINT verified_contexts_assertion_id_unique UNIQUE (assertion_id)
);

ALTER TABLE stonez_context_private.verification_keys OWNER TO stonez_bulk_context_owner;
ALTER TABLE stonez_context_private.verified_contexts OWNER TO stonez_bulk_context_owner;
REVOKE ALL ON ALL TABLES IN SCHEMA stonez_context_private FROM PUBLIC, stonez_bulk_worker, stonez_bulk_context_key_admin;
GRANT SELECT ON stonez_context_private.verification_keys TO stonez_bulk_context_owner;
GRANT SELECT, INSERT ON stonez_context_private.verified_contexts TO stonez_bulk_context_owner;

COMMIT;
