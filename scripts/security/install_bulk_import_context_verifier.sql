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


CREATE OR REPLACE FUNCTION stonez_context_private.decode_base64url(p_value text)
RETURNS bytea
LANGUAGE plpgsql
IMMUTABLE
STRICT
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_decoded bytea;
    v_canonical text;
BEGIN
    IF p_value = '' OR p_value !~ '^[A-Za-z0-9_-]+
 OR length(p_value) % 4 = 1 THEN
        RAISE EXCEPTION 'invalid compact JWS base64url segment' USING ERRCODE = '22023';
    END IF;
    v_decoded := decode(
        translate(p_value, '-_', '+/') || repeat('=', (4 - length(p_value) % 4) % 4),
        'base64'
    );
    v_canonical := rtrim(
        translate(replace(encode(v_decoded, 'base64'), E'\n', ''), '+/', '-_'),
        '='
    );
    IF v_canonical <> p_value THEN
        RAISE EXCEPTION 'non-canonical compact JWS base64url segment' USING ERRCODE = '22023';
    END IF;
    RETURN v_decoded;
END
$fn$;
ALTER FUNCTION stonez_context_private.decode_base64url(text) OWNER TO stonez_bulk_context_owner;
REVOKE ALL ON FUNCTION stonez_context_private.decode_base64url(text) FROM PUBLIC;


CREATE OR REPLACE FUNCTION stonez_context_private.constant_time_equal(p_left bytea, p_right bytea)
RETURNS boolean
LANGUAGE plpgsql
IMMUTABLE
STRICT
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_diff integer := 0;
    v_i integer;
BEGIN
    IF length(p_left) <> 32 OR length(p_right) <> 32 THEN
        RETURN false;
    END IF;
    FOR v_i IN 0..31 LOOP
        v_diff := v_diff | (get_byte(p_left, v_i) # get_byte(p_right, v_i));
    END LOOP;
    RETURN v_diff = 0;
END
$fn$;
ALTER FUNCTION stonez_context_private.constant_time_equal(bytea,bytea) OWNER TO stonez_bulk_context_owner;
REVOKE ALL ON FUNCTION stonez_context_private.constant_time_equal(bytea,bytea) FROM PUBLIC;

COMMIT;
