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
'55555555-5555-4555-8555-555555555555','running',clock_timestamp()),
('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','22222222-2222-4222-8222-222222222222',
'33333333-3333-4333-8333-333333333333','dddddddd-dddd-4ddd-8ddd-dddddddddddd',
'eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee','running',clock_timestamp());
CREATE TABLE public.users(id uuid PRIMARY KEY, school_id uuid, email text);
CREATE TABLE public.students(id uuid PRIMARY KEY, school_id uuid NOT NULL);
INSERT INTO public.students(id,school_id) VALUES
('21212121-2121-4212-8212-212121212121','22222222-2222-4222-8222-222222222222'),
('23232323-2323-4323-8323-232323232323','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb');
CREATE TABLE public.teacher_profiles(id uuid PRIMARY KEY, school_id uuid NOT NULL);
CREATE TABLE public.parent_profiles(id uuid PRIMARY KEY, school_id uuid NOT NULL);
CREATE TABLE public.academic_sessions(id uuid PRIMARY KEY, school_id uuid NOT NULL);
CREATE TABLE public.school_classes(id uuid PRIMARY KEY, school_id uuid NOT NULL);
CREATE TABLE public.sections(id uuid PRIMARY KEY, school_id uuid NOT NULL);
CREATE TABLE public.student_enrollments(id uuid PRIMARY KEY, school_id uuid NOT NULL);
CREATE TABLE public.guardian_relationships(id uuid PRIMARY KEY, school_id uuid NOT NULL);
SQL

psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_verifier.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_worker_policies.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE stonez_bulk_worker_processing_test LOGIN PASSWORD 'bulk_context_worker_ci_password' NOSUPERUSER NOBYPASSRLS INHERIT;
GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_test;
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
    "$WORKER_ID" "$1" "$2" "$2" "$3" "$JOB_ID" "$4" "$INITIATOR_ID" "$WORKER_ID" "$5"
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
TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '66666666-6666-4666-8666-666666666666' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"

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
    RAISE EXCEPTION 'worker RLS did not scope the queue to the exact signed job';
  END IF;
  IF (SELECT count(*) FROM public.students) <> 1 THEN
    RAISE EXCEPTION 'worker RLS exposed a student from another tenant';
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
REPLAY="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '77777777-7777-4777-8777-777777777777' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$REPLAY" <<'SQL' >/dev/null
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
COMMIT;
SQL
expect_rejected "$REPLAY" "committed replay"
FORGED="$(printf '%s' "$TOKEN" | sed 's/.$/A/')"
expect_rejected "$FORGED" "modified signature"
EXPIRED="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '88888888-8888-4888-8888-888888888888' "$((NOW-120))" "$((NOW-90))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$EXPIRED" "expired assertion"
UNKNOWN="$(make_token '{"alg":"HS256","kid":"unknown-key","typ":"JWT"}' "$(payload '99999999-9999-4999-8999-999999999999' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$UNKNOWN" "unknown key ID"
ALG_NONE="$(make_token '{"alg":"none","kid":"ci-key-1","typ":"JWT"}' "$(payload '10101010-1010-4010-8010-101010101010' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$ALG_NONE" "algorithm substitution"
ALT_KEY_HEX="$(printf 'cd%.0s' {1..32})"
WRONG_KEY="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '11111111-1111-4111-8111-111111111112' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$ALT_KEY_HEX")"
expect_rejected "$WRONG_KEY" "untrusted signing key"
CROSS_TENANT="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '12121212-1212-4212-8212-121212121212' "$NOW" "$((NOW+30))" 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb' "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$CROSS_TENANT" "cross-tenant school claim"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "INSERT INTO public.students(id,school_id) VALUES ('24242424-2424-4424-8424-242424242424','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb')" >/dev/null 2>&1; then
  echo "Worker unexpectedly inserted a cross-tenant student" >&2
  exit 1
fi
echo "Rejected cross-tenant INSERT by worker RLS"

STALE_LEASE="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '13131313-1313-4313-8313-131313131313' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "UPDATE public.bulk_import_jobs SET lease_id='25252525-2525-4525-8525-252525252525' WHERE id='$JOB_ID'"
expect_rejected "$STALE_LEASE" "replaced lease generation"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "UPDATE public.bulk_import_jobs SET lease_id='$LEASE_ID' WHERE id='$JOB_ID'"

CONCURRENT="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '14141414-1414-4414-8414-141414141414' "$NOW" "$((NOW+30))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
(psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$CONCURRENT" <<'SQL'
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
SELECT pg_sleep(2);
COMMIT;
SQL
) &
first_pid=$!
sleep 0.3
expect_rejected "$CONCURRENT" "concurrent replay after row-lock serialization"
wait "$first_pid"

echo "PostgreSQL signed-context verifier adversarial tests passed."
