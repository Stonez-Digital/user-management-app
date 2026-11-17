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

-- The operator/table owner grants the verifier owner access to the queue
-- relation before switching to the dedicated NOLOGIN owner.
GRANT USAGE ON SCHEMA public TO stonez_bulk_context_owner;
GRANT SELECT ON public.bulk_import_jobs TO stonez_bulk_context_owner;
GRANT UPDATE (lease_id) ON public.bulk_import_jobs TO stonez_bulk_context_owner;

-- The role creator receives a non-inheriting membership. Switch into the
-- dedicated owner before creating private objects and SECURITY DEFINER code.
GRANT stonez_bulk_context_owner TO CURRENT_USER WITH INHERIT FALSE, SET TRUE;
SET LOCAL ROLE stonez_bulk_context_owner;

CREATE SCHEMA IF NOT EXISTS stonez_context_private AUTHORIZATION CURRENT_USER;
ALTER SCHEMA stonez_context_private OWNER TO CURRENT_USER;
REVOKE ALL ON SCHEMA stonez_context_private FROM PUBLIC;
GRANT USAGE ON SCHEMA stonez_context_private TO stonez_bulk_worker, stonez_bulk_context_key_admin;

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
    IF p_value = '' OR p_value ~ '[^A-Za-z0-9_-]' OR length(p_value) % 4 = 1 THEN
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
REVOKE ALL ON FUNCTION stonez_context_private.constant_time_equal(bytea,bytea) FROM PUBLIC;


CREATE OR REPLACE FUNCTION stonez_context_private.verify_bulk_import_context(p_token text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_parts text[];
    v_header json;
    v_payload json;
    v_count integer;
    v_distinct integer;
    v_kid text;
    v_key bytea;
    v_ext_schema text;
    v_signature bytea;
    v_expected bytea;
    v_now timestamptz;
    v_iat numeric;
    v_nbf numeric;
    v_exp numeric;
    v_job uuid;
    v_school uuid;
    v_initiator uuid;
    v_worker uuid;
    v_lease uuid;
    v_jti uuid;
    v_queue public.bulk_import_jobs%ROWTYPE;
BEGIN
    IF NOT pg_has_role(session_user, 'stonez_bulk_worker', 'MEMBER')
       OR EXISTS (SELECT 1 FROM pg_roles WHERE rolname = session_user
                  AND (rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb))
       OR pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER')
       OR pg_has_role(session_user, 'stonez_bulk_queue_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'login is not an eligible restricted bulk worker' USING ERRCODE = '42501';
    END IF;
    IF p_token IS NULL OR length(p_token) > 4096 THEN
        RAISE EXCEPTION 'missing or oversized bulk worker assertion' USING ERRCODE = '22023';
    END IF;

    v_parts := string_to_array(p_token, '.');
    IF cardinality(v_parts) <> 3 OR v_parts[1] = '' OR v_parts[2] = '' OR v_parts[3] = '' THEN
        RAISE EXCEPTION 'malformed compact JWS' USING ERRCODE = '22023';
    END IF;
    v_header := convert_from(stonez_context_private.decode_base64url(v_parts[1]), 'UTF8')::json;
    v_payload := convert_from(stonez_context_private.decode_base64url(v_parts[2]), 'UTF8')::json;
    v_signature := stonez_context_private.decode_base64url(v_parts[3]);
    IF json_typeof(v_header) <> 'object' OR json_typeof(v_payload) <> 'object' THEN
        RAISE EXCEPTION 'JWS header and claims must be JSON objects' USING ERRCODE = '22023';
    END IF;

    SELECT count(*), count(DISTINCT key) INTO v_count, v_distinct FROM json_each(v_header);
    IF v_count <> 3 OR v_distinct <> 3
       OR (SELECT count(*) FROM json_each(v_header) WHERE key IN ('alg','kid','typ')) <> 3
       OR v_header->>'alg' <> 'HS256' OR v_header->>'typ' <> 'JWT' THEN
        RAISE EXCEPTION 'unsupported, duplicate, or unexpected JWS header members' USING ERRCODE = '22023';
    END IF;
    v_kid := v_header->>'kid';
    IF v_kid IS NULL OR v_kid = '' OR length(v_kid) > 64 THEN
        RAISE EXCEPTION 'invalid JWS key ID' USING ERRCODE = '22023';
    END IF;

    SELECT count(*), count(DISTINCT key) INTO v_count, v_distinct FROM json_each(v_payload);
    IF v_count <> 12 OR v_distinct <> 12
       OR (SELECT count(*) FROM json_each(v_payload) WHERE key = ANY
           (ARRAY['iss','sub','aud','jti','iat','nbf','exp','job_id','school_id','initiated_by','worker_id','lease_id'])) <> 12 THEN
        RAISE EXCEPTION 'missing, duplicate, or unexpected assertion claims' USING ERRCODE = '22023';
    END IF;
    IF v_payload->>'iss' <> 'stonez-bulk-import-worker'
       OR v_payload->>'aud' <> 'stonez-postgres-bulk-import-context' THEN
        RAISE EXCEPTION 'invalid assertion issuer or audience' USING ERRCODE = '22023';
    END IF;
    IF json_typeof(v_payload->'iat') IS DISTINCT FROM 'number'
       OR json_typeof(v_payload->'nbf') IS DISTINCT FROM 'number'
       OR json_typeof(v_payload->'exp') IS DISTINCT FROM 'number' THEN
        RAISE EXCEPTION 'invalid assertion time claims' USING ERRCODE = '22023';
    END IF;
    v_iat := (v_payload->>'iat')::numeric;
    v_nbf := (v_payload->>'nbf')::numeric;
    v_exp := (v_payload->>'exp')::numeric;

    IF json_typeof(v_payload->'job_id') IS DISTINCT FROM 'string'
       OR json_typeof(v_payload->'school_id') IS DISTINCT FROM 'string'
       OR json_typeof(v_payload->'initiated_by') IS DISTINCT FROM 'string'
       OR json_typeof(v_payload->'worker_id') IS DISTINCT FROM 'string'
       OR json_typeof(v_payload->'lease_id') IS DISTINCT FROM 'string'
       OR json_typeof(v_payload->'jti') IS DISTINCT FROM 'string'
       OR length(v_payload->>'job_id') <> 36
       OR (v_payload->>'job_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
       OR length(v_payload->>'school_id') <> 36
       OR (v_payload->>'school_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
       OR length(v_payload->>'initiated_by') <> 36
       OR (v_payload->>'initiated_by') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
       OR length(v_payload->>'worker_id') <> 36
       OR (v_payload->>'worker_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
       OR length(v_payload->>'lease_id') <> 36
       OR (v_payload->>'lease_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
       OR length(v_payload->>'jti') <> 36
       OR (v_payload->>'jti') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' THEN
        RAISE EXCEPTION 'invalid canonical UUID assertion claim' USING ERRCODE = '22023';
    END IF;
    v_job := (v_payload->>'job_id')::uuid;
    v_school := (v_payload->>'school_id')::uuid;
    v_initiator := (v_payload->>'initiated_by')::uuid;
    v_worker := (v_payload->>'worker_id')::uuid;
    v_lease := (v_payload->>'lease_id')::uuid;
    v_jti := (v_payload->>'jti')::uuid;
    IF v_payload->>'sub' <> v_worker::text OR length(v_signature) <> 32 THEN
        RAISE EXCEPTION 'invalid assertion subject or HS256 signature length' USING ERRCODE = '22023';
    END IF;

    SELECT key_bytes INTO v_key
      FROM stonez_context_private.verification_keys
     WHERE key_id = v_kid AND not_before <= clock_timestamp()
       AND (not_after IS NULL OR not_after > clock_timestamp()) AND revoked_at IS NULL
     FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'unknown, inactive, or revoked assertion key' USING ERRCODE = '28000';
    END IF;

    SELECT namespace.nspname INTO v_ext_schema
      FROM pg_extension AS extension
      JOIN pg_namespace AS namespace ON namespace.oid = extension.extnamespace
      JOIN pg_proc AS procedure ON procedure.pronamespace = namespace.oid
     WHERE extension.extname = 'pgcrypto' AND procedure.proname = 'hmac'
       AND pg_get_function_identity_arguments(procedure.oid) = 'bytea, bytea, text'
     LIMIT 1;
    IF v_ext_schema IS NULL THEN
        RAISE EXCEPTION 'pgcrypto.hmac(bytea, bytea, text) is unavailable' USING ERRCODE = '55000';
    END IF;
    EXECUTE format('SELECT %I.hmac($1, $2, $3)', v_ext_schema)
       INTO v_expected
       USING convert_to(v_parts[1] || '.' || v_parts[2], 'UTF8'), v_key, 'sha256';
    IF NOT stonez_context_private.constant_time_equal(v_signature, v_expected) THEN
        RAISE EXCEPTION 'invalid bulk worker assertion signature' USING ERRCODE = '28000';
    END IF;

    v_now := clock_timestamp();
    IF v_iat > extract(epoch FROM v_now) + 5
       OR v_nbf > extract(epoch FROM v_now) + 5
       OR v_exp <= extract(epoch FROM v_now) - 5
       OR v_exp <= v_iat OR v_exp - v_iat > 60 THEN
        RAISE EXCEPTION 'bulk worker assertion is outside its permitted time window' USING ERRCODE = '28000';
    END IF;

    SELECT * INTO v_queue FROM public.bulk_import_jobs WHERE id = v_job FOR UPDATE;
    v_now := clock_timestamp();
    IF NOT FOUND OR v_queue.school_id <> v_school
       OR v_queue.initiated_by <> v_initiator OR v_queue.worker_id <> v_worker
       OR v_queue.lease_id <> v_lease OR v_queue.status <> 'running'
       OR v_queue.claimed_at IS NULL
       OR v_queue.claimed_at < v_now - interval '5 minutes' THEN
        RAISE EXCEPTION 'assertion does not match a current, unexpired queue lease' USING ERRCODE = '28000';
    END IF;

    INSERT INTO stonez_context_private.verified_contexts (
        backend_pid, transaction_id, assertion_id, job_id, school_id,
        initiated_by, worker_id, lease_id, verified_at
    ) VALUES (
        pg_backend_pid(), pg_current_xact_id(), v_jti, v_job, v_school,
        v_initiator, v_worker, v_lease, v_now
    );
EXCEPTION
    WHEN unique_violation THEN
        RAISE EXCEPTION 'bulk worker assertion replay or duplicate transaction context' USING ERRCODE = '28000';
END
$fn$;
REVOKE ALL ON FUNCTION stonez_context_private.verify_bulk_import_context(text) FROM PUBLIC;
GRANT SELECT ON public.bulk_import_jobs TO stonez_bulk_context_owner;
GRANT UPDATE (lease_id) ON public.bulk_import_jobs TO stonez_bulk_context_owner;
DROP POLICY IF EXISTS bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs
    FOR SELECT TO stonez_bulk_context_owner USING (true);

COMMIT;
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
REVOKE ALL ON FUNCTION stonez_context_private.constant_time_equal(bytea,bytea) FROM PUBLIC;


CREATE OR REPLACE FUNCTION stonez_context_private.verify_bulk_import_context(p_token text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_parts text[];
    v_header json;
    v_payload json;
    v_count integer;
    v_distinct integer;
    v_kid text;
    v_key bytea;
    v_ext_schema text;
    v_signature bytea;
    v_expected bytea;
    v_now timestamptz;
    v_iat numeric;
    v_nbf numeric;
    v_exp numeric;
    v_job uuid;
    v_school uuid;
    v_initiator uuid;
    v_worker uuid;
    v_lease uuid;
    v_jti uuid;
    v_queue public.bulk_import_jobs%ROWTYPE;
BEGIN
    IF NOT pg_has_role(session_user, 'stonez_bulk_worker', 'MEMBER')
       OR EXISTS (SELECT 1 FROM pg_roles WHERE rolname = session_user
                  AND (rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb))
       OR pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER')
       OR pg_has_role(session_user, 'stonez_bulk_queue_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'login is not an eligible restricted bulk worker' USING ERRCODE = '42501';
    END IF;
    IF p_token IS NULL OR length(p_token) > 4096 THEN
        RAISE EXCEPTION 'missing or oversized bulk worker assertion' USING ERRCODE = '22023';
    END IF;

    v_parts := string_to_array(p_token, '.');
    IF cardinality(v_parts) <> 3 OR v_parts[1] = '' OR v_parts[2] = '' OR v_parts[3] = '' THEN
        RAISE EXCEPTION 'malformed compact JWS' USING ERRCODE = '22023';
    END IF;
    v_header := convert_from(stonez_context_private.decode_base64url(v_parts[1]), 'UTF8')::json;
    v_payload := convert_from(stonez_context_private.decode_base64url(v_parts[2]), 'UTF8')::json;
    v_signature := stonez_context_private.decode_base64url(v_parts[3]);
    IF json_typeof(v_header) <> 'object' OR json_typeof(v_payload) <> 'object' THEN
        RAISE EXCEPTION 'JWS header and claims must be JSON objects' USING ERRCODE = '22023';
    END IF;

    SELECT count(*), count(DISTINCT key) INTO v_count, v_distinct FROM json_each(v_header);
    IF v_count <> 3 OR v_distinct <> 3
       OR (SELECT count(*) FROM json_each(v_header) WHERE key IN ('alg','kid','typ')) <> 3
       OR v_header->>'alg' <> 'HS256' OR v_header->>'typ' <> 'JWT' THEN
        RAISE EXCEPTION 'unsupported, duplicate, or unexpected JWS header members' USING ERRCODE = '22023';
    END IF;
    v_kid := v_header->>'kid';
    IF v_kid IS NULL OR v_kid = '' OR length(v_kid) > 64 THEN
        RAISE EXCEPTION 'invalid JWS key ID' USING ERRCODE = '22023';
    END IF;

    SELECT count(*), count(DISTINCT key) INTO v_count, v_distinct FROM json_each(v_payload);
    IF v_count <> 12 OR v_distinct <> 12
       OR (SELECT count(*) FROM json_each(v_payload) WHERE key = ANY
           (ARRAY['iss','sub','aud','jti','iat','nbf','exp','job_id','school_id','initiated_by','worker_id','lease_id'])) <> 12 THEN
        RAISE EXCEPTION 'missing, duplicate, or unexpected assertion claims' USING ERRCODE = '22023';
    END IF;
    IF v_payload->>'iss' <> 'stonez-bulk-import-worker'
       OR v_payload->>'aud' <> 'stonez-postgres-bulk-import-context' THEN
        RAISE EXCEPTION 'invalid assertion issuer or audience' USING ERRCODE = '22023';
    END IF;
    IF json_typeof(v_payload->'iat') <> 'number'
       OR json_typeof(v_payload->'nbf') <> 'number'
       OR json_typeof(v_payload->'exp') <> 'number'
       OR (v_payload->>'iat') !~ '^[0-9]+([.][0-9]+)?

       OR (v_payload->>'nbf') !~ '^[0-9]+([.][0-9]+)?

       OR (v_payload->>'exp') !~ '^[0-9]+([.][0-9]+)?
 THEN
        RAISE EXCEPTION 'invalid assertion time claims' USING ERRCODE = '22023';
    END IF;
    v_iat := (v_payload->>'iat')::numeric;
    v_nbf := (v_payload->>'nbf')::numeric;
    v_exp := (v_payload->>'exp')::numeric;

    IF (v_payload->>'job_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}

       OR (v_payload->>'school_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}

       OR (v_payload->>'initiated_by') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}

       OR (v_payload->>'worker_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}

       OR (v_payload->>'lease_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}

       OR (v_payload->>'jti') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}
 THEN
        RAISE EXCEPTION 'invalid canonical UUID assertion claim' USING ERRCODE = '22023';
    END IF;
    v_job := (v_payload->>'job_id')::uuid;
    v_school := (v_payload->>'school_id')::uuid;
    v_initiator := (v_payload->>'initiated_by')::uuid;
    v_worker := (v_payload->>'worker_id')::uuid;
    v_lease := (v_payload->>'lease_id')::uuid;
    v_jti := (v_payload->>'jti')::uuid;
    IF v_payload->>'sub' <> v_worker::text OR length(v_signature) <> 32 THEN
        RAISE EXCEPTION 'invalid assertion subject or HS256 signature length' USING ERRCODE = '22023';
    END IF;

    SELECT key_bytes INTO v_key
      FROM stonez_context_private.verification_keys
     WHERE key_id = v_kid AND not_before <= clock_timestamp()
       AND (not_after IS NULL OR not_after > clock_timestamp()) AND revoked_at IS NULL
     FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'unknown, inactive, or revoked assertion key' USING ERRCODE = '28000';
    END IF;

    SELECT namespace.nspname INTO v_ext_schema
      FROM pg_extension AS extension
      JOIN pg_namespace AS namespace ON namespace.oid = extension.extnamespace
      JOIN pg_proc AS procedure ON procedure.pronamespace = namespace.oid
     WHERE extension.extname = 'pgcrypto' AND procedure.proname = 'hmac'
       AND pg_get_function_identity_arguments(procedure.oid) = 'bytea, bytea, text'
     LIMIT 1;
    IF v_ext_schema IS NULL THEN
        RAISE EXCEPTION 'pgcrypto.hmac(bytea, bytea, text) is unavailable' USING ERRCODE = '55000';
    END IF;
    EXECUTE format('SELECT %I.hmac($1, $2, $3)', v_ext_schema)
       INTO v_expected
       USING convert_to(v_parts[1] || '.' || v_parts[2], 'UTF8'), v_key, 'sha256';
    IF NOT stonez_context_private.constant_time_equal(v_signature, v_expected) THEN
        RAISE EXCEPTION 'invalid bulk worker assertion signature' USING ERRCODE = '28000';
    END IF;

    v_now := clock_timestamp();
    IF v_iat > extract(epoch FROM v_now) + 5
       OR v_nbf > extract(epoch FROM v_now) + 5
       OR v_exp <= extract(epoch FROM v_now) - 5
       OR v_exp <= v_iat OR v_exp - v_iat > 60 THEN
        RAISE EXCEPTION 'bulk worker assertion is outside its permitted time window' USING ERRCODE = '28000';
    END IF;

    SELECT * INTO v_queue FROM public.bulk_import_jobs WHERE id = v_job FOR UPDATE;
    v_now := clock_timestamp();
    IF NOT FOUND OR v_queue.school_id <> v_school
       OR v_queue.initiated_by <> v_initiator OR v_queue.worker_id <> v_worker
       OR v_queue.lease_id <> v_lease OR v_queue.status <> 'running'
       OR v_queue.claimed_at IS NULL
       OR v_queue.claimed_at < v_now - interval '5 minutes' THEN
        RAISE EXCEPTION 'assertion does not match a current, unexpired queue lease' USING ERRCODE = '28000';
    END IF;

    INSERT INTO stonez_context_private.verified_contexts (
        backend_pid, transaction_id, assertion_id, job_id, school_id,
        initiated_by, worker_id, lease_id, verified_at
    ) VALUES (
        pg_backend_pid(), pg_current_xact_id(), v_jti, v_job, v_school,
        v_initiator, v_worker, v_lease, v_now
    );
EXCEPTION
    WHEN unique_violation THEN
        RAISE EXCEPTION 'bulk worker assertion replay or duplicate transaction context' USING ERRCODE = '28000';
END
$fn$;
REVOKE ALL ON FUNCTION stonez_context_private.verify_bulk_import_context(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION stonez_context_private.current_worker_school_id()
RETURNS uuid
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
    SELECT context.school_id
      FROM stonez_context_private.verified_contexts AS context
     WHERE context.backend_pid = pg_backend_pid()
       AND context.transaction_id = pg_current_xact_id()
       AND pg_has_role(session_user, 'stonez_bulk_worker', 'MEMBER')
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.current_worker_initiator_id()
RETURNS uuid
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
    SELECT context.initiated_by
      FROM stonez_context_private.verified_contexts AS context
     WHERE context.backend_pid = pg_backend_pid()
       AND context.transaction_id = pg_current_xact_id()
       AND pg_has_role(session_user, 'stonez_bulk_worker', 'MEMBER')
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.current_worker_job_id()
RETURNS uuid
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
    SELECT context.job_id
      FROM stonez_context_private.verified_contexts AS context
     WHERE context.backend_pid = pg_backend_pid()
       AND context.transaction_id = pg_current_xact_id()
       AND pg_has_role(session_user, 'stonez_bulk_worker', 'MEMBER')
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.register_verification_key(
    p_key_id text, p_key_bytes bytea, p_not_before timestamptz, p_not_after timestamptz DEFAULT NULL
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    IF NOT pg_has_role(session_user, 'stonez_bulk_context_key_admin', 'MEMBER') THEN
        RAISE EXCEPTION 'caller is not authorized to register verifier keys' USING ERRCODE = '42501';
    END IF;
    IF p_key_id IS NULL OR p_key_id = '' OR length(p_key_id) > 64
       OR p_key_bytes IS NULL OR octet_length(p_key_bytes) < 32
       OR p_not_before IS NULL
       OR (p_not_after IS NOT NULL AND p_not_after <= p_not_before) THEN
        RAISE EXCEPTION 'invalid verifier key registration parameters' USING ERRCODE = '22023';
    END IF;
    INSERT INTO stonez_context_private.verification_keys(key_id,key_bytes,not_before,not_after)
    VALUES (p_key_id,p_key_bytes,p_not_before,p_not_after);
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.retire_verification_key(
    p_key_id text, p_not_after timestamptz
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    IF NOT pg_has_role(session_user, 'stonez_bulk_context_key_admin', 'MEMBER') THEN
        RAISE EXCEPTION 'caller is not authorized to retire verifier keys' USING ERRCODE = '42501';
    END IF;
    UPDATE stonez_context_private.verification_keys
       SET not_after = p_not_after
     WHERE key_id = p_key_id AND revoked_at IS NULL AND p_not_after > not_before;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'verifier key not found or retirement time invalid' USING ERRCODE = '22023';
    END IF;
END
$fn$;

CREATE OR REPLACE FUNCTION stonez_context_private.revoke_verification_key(p_key_id text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    IF NOT pg_has_role(session_user, 'stonez_bulk_context_key_admin', 'MEMBER') THEN
        RAISE EXCEPTION 'caller is not authorized to revoke verifier keys' USING ERRCODE = '42501';
    END IF;
    UPDATE stonez_context_private.verification_keys
       SET revoked_at = clock_timestamp()
     WHERE key_id = p_key_id AND revoked_at IS NULL;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'active verifier key not found' USING ERRCODE = '22023';
    END IF;
END
$fn$;

REVOKE ALL ON ALL FUNCTIONS IN SCHEMA stonez_context_private FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_context_private.verify_bulk_import_context(text) TO stonez_bulk_worker;
GRANT EXECUTE ON FUNCTION stonez_context_private.current_worker_school_id() TO stonez_bulk_worker;
GRANT EXECUTE ON FUNCTION stonez_context_private.current_worker_initiator_id() TO stonez_bulk_worker;
GRANT EXECUTE ON FUNCTION stonez_context_private.current_worker_job_id() TO stonez_bulk_worker;
GRANT EXECUTE ON FUNCTION stonez_context_private.register_verification_key(text,bytea,timestamptz,timestamptz) TO stonez_bulk_context_key_admin;
GRANT EXECUTE ON FUNCTION stonez_context_private.retire_verification_key(text,timestamptz) TO stonez_bulk_context_key_admin;
GRANT EXECUTE ON FUNCTION stonez_context_private.revoke_verification_key(text) TO stonez_bulk_context_key_admin;

-- RLS policies must be installed by the application-table owner, not the
-- verifier function owner. The processing role has no membership in the API
-- or queue-owner groups, so raw tenant GUC policies cannot act as fallback.
RESET ROLE;
ALTER TABLE public.bulk_import_jobs ENABLE ROW LEVEL SECURITY;
GRANT SELECT ON public.bulk_import_jobs TO stonez_bulk_worker;
GRANT UPDATE (status,total,valid,created,updated,skipped,failed,credentials_sent,credential_emails_failed,errors_json,claimed_at,worker_id,last_error) ON public.bulk_import_jobs TO stonez_bulk_worker;
DROP POLICY IF EXISTS bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs
    FOR SELECT TO stonez_bulk_context_owner USING (true);
DROP POLICY IF EXISTS bulk_import_jobs_signed_worker_select ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_signed_worker_select ON public.bulk_import_jobs
    FOR SELECT TO stonez_bulk_worker
    USING (
        school_id = stonez_context_private.current_worker_school_id()
        AND initiated_by = stonez_context_private.current_worker_initiator_id()
    );
DROP POLICY IF EXISTS bulk_import_jobs_signed_worker_update ON public.bulk_import_jobs;
CREATE POLICY bulk_import_jobs_signed_worker_update ON public.bulk_import_jobs
    FOR UPDATE TO stonez_bulk_worker
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
COMMIT;
