#!/usr/bin/env bash
set -euo pipefail

: "$BULK_CONTEXT_TEST_DATABASE_URL"
: "$BULK_CONTEXT_TEST_WORKER_URL"
: "$BULK_CONTEXT_TEST_DATABASE"
runner_url="$BULK_CONTEXT_TEST_DATABASE_URL"
worker_url="$BULK_CONTEXT_TEST_WORKER_URL"
actual_db="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c 'SELECT current_database()')"
if [[ "$actual_db" != "$BULK_CONTEXT_TEST_DATABASE" || "$actual_db" == *prod* || "$actual_db" == *production* ]]; then
  echo "Refusing verifier fixture against database '$actual_db'" >&2
  exit 1
fi

psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE stonez_api_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
CREATE ROLE stonez_bulk_queue_owner NOLOGIN NOSUPERUSER NOBYPASSRLS;
CREATE SCHEMA pgcrypto_probe;
CREATE EXTENSION pgcrypto WITH SCHEMA pgcrypto_probe;
CREATE TABLE public.bulk_import_jobs (
    id uuid PRIMARY KEY, school_id uuid NOT NULL, initiated_by uuid NOT NULL,
    worker_id uuid, lease_id uuid, status text NOT NULL, claimed_at timestamptz,
    total integer NOT NULL DEFAULT 0, valid integer NOT NULL DEFAULT 0,
    created integer NOT NULL DEFAULT 0, updated integer NOT NULL DEFAULT 0,
    skipped integer NOT NULL DEFAULT 0, failed integer NOT NULL DEFAULT 0,
    credentials_sent integer NOT NULL DEFAULT 0, credential_emails_failed integer NOT NULL DEFAULT 0,
    errors_json jsonb NOT NULL DEFAULT '[]'::jsonb, last_error text NOT NULL DEFAULT ''
);
INSERT INTO public.bulk_import_jobs(id,school_id,initiated_by,worker_id,lease_id,status,claimed_at)
VALUES ('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222',
'33333333-3333-4333-8333-333333333333','44444444-4444-4444-8444-444444444444',
'55555555-5555-4555-8555-555555555555','running',clock_timestamp());
SQL

psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_verifier.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE stonez_bulk_worker_test LOGIN PASSWORD 'bulk_context_worker_ci_password' NOSUPERUSER NOBYPASSRLS INHERIT;
GRANT stonez_bulk_worker TO stonez_bulk_worker_test;
SELECT stonez_context_private.register_verification_key(
  'ci-key-1', decode(repeat('ab',32),'hex'), clock_timestamp() - interval '5 minutes', NULL
);
SQL

KEY_HEX="$(printf 'ab%.0s' {1..32})"
JOB_ID="11111111-1111-4111-8111-111111111111"
SCHOOL_ID="22222222-2222-4222-8222-222222222222"
INITIATOR_ID="33333333-3333-4333-8333-333333333333"
WORKER_ID="44444444-4444-4444-8444-444444444444"
LEASE_ID="55555555-5555-4555-8555-555555555555"
b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
make_token() {
  local header="$1" payload="$2" key_hex="$3" h p sig
  h="$(printf '%s' "$header" | b64url)"
  p="$(printf '%s' "$payload" | b64url)"
  sig="$(printf '%s' "$h.$p" | openssl dgst -sha256 -mac HMAC -macopt "hexkey:$key_hex" -binary | b64url)"
  printf '%s.%s.%s' "$h" "$p" "$sig"
}
payload() {
  printf '{"iss":"stonez-bulk-import-worker","sub":"%s","aud":"stonez-postgres-bulk-import-context","jti":"%s","iat":%s,"nbf":%s,"exp":%s,"job_id":"%s","school_id":"%s","initiated_by":"%s","worker_id":"%s","lease_id":"%s"}' \
    "$WORKER_ID" "$1" "$2" "$2" "$3" "$JOB_ID" "$SCHOOL_ID" "$INITIATOR_ID" "$WORKER_ID" "$LEASE_ID"
}
expect_rejected() {
  if psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$1" <<'SQL' >/dev/null 2>&1
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
COMMIT;
SQL
  then echo "Verifier unexpectedly accepted $2" >&2; exit 1; fi
  echo "Rejected $2"
}
NOW="$(date +%s)"
TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '66666666-6666-4666-8666-666666666666' "$NOW" "$((NOW+30))")" "$KEY_HEX")"

echo "== Valid assertion, transaction binding, rollback retry, and tenant isolation =="
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$TOKEN" <<'SQL'
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
DO $$
BEGIN
  IF stonez_context_private.current_worker_school_id() <> '22222222-2222-4222-8222-222222222222'::uuid THEN
    RAISE EXCEPTION 'transaction-bound accessor returned wrong tenant';
  END IF;
  IF (SELECT count(*) FROM public.bulk_import_jobs) <> 1 THEN
    RAISE EXCEPTION 'worker RLS did not scope the queue to the signed tenant';
  END IF;
END
$$;
ROLLBACK;
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
COMMIT;
DO $$
BEGIN
  IF stonez_context_private.current_worker_school_id() IS NOT NULL THEN
    RAISE EXCEPTION 'verified context leaked beyond its transaction';
  END IF;
END
$$;
SQL

echo "== Replay, forgery, expiry, and key selection =="
REPLAY="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '77777777-7777-4777-8777-777777777777' "$NOW" "$((NOW+30))")" "$KEY_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$REPLAY" -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context(:'token'); COMMIT;" >/dev/null
expect_rejected "$REPLAY" "committed replay"
FORGED="$(printf '%s' "$TOKEN" | sed 's/.$/A/')"
expect_rejected "$FORGED" "modified signature"
EXPIRED="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '88888888-8888-4888-8888-888888888888' "$((NOW-120))" "$((NOW-90))")" "$KEY_HEX")"
expect_rejected "$EXPIRED" "expired assertion"
UNKNOWN="$(make_token '{"alg":"HS256","kid":"unknown-key","typ":"JWT"}' "$(payload '99999999-9999-4999-8999-999999999999' "$NOW" "$((NOW+30))")" "$KEY_HEX")"
expect_rejected "$UNKNOWN" "unknown key ID"
echo "PostgreSQL signed-context verifier smoke/adversarial tests passed."
