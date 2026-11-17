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
CREATE ROLE stonez_bulk_worker LOGIN PASSWORD 'bulk_context_claim_ci_password' NOSUPERUSER NOBYPASSRLS INHERIT;
GRANT stonez_api_runtime TO stonez_bulk_worker;
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
CREATE TABLE public.users(id uuid PRIMARY KEY, school_id uuid, email text, role text, password_hash text);
CREATE TABLE public.students(id uuid PRIMARY KEY, school_id uuid NOT NULL, admission_number text);
INSERT INTO public.students(id,school_id) VALUES
('21212121-2121-4212-8212-212121212121','22222222-2222-4222-8222-222222222222'),
('23232323-2323-4323-8323-232323232323','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb');
CREATE TABLE public.teacher_profiles(id uuid PRIMARY KEY, school_id uuid NOT NULL, staff_id text);
CREATE TABLE public.parent_profiles(id uuid PRIMARY KEY, school_id uuid NOT NULL, parent_identifier text, user_id uuid);
CREATE TABLE public.academic_sessions(id uuid PRIMARY KEY, school_id uuid NOT NULL, name text, status text);
CREATE TABLE public.school_classes(id uuid PRIMARY KEY, school_id uuid NOT NULL, name text);
CREATE TABLE public.sections(id uuid PRIMARY KEY, school_id uuid NOT NULL, class_id uuid, name text);
CREATE TABLE public.student_enrollments(id uuid PRIMARY KEY, school_id uuid NOT NULL, student_id uuid, academic_session_id uuid, status text);
CREATE TABLE public.guardian_relationships(id uuid PRIMARY KEY, school_id uuid NOT NULL, guardian_user_id uuid, student_id uuid);
SQL

psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_verifier.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_worker_policies.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE stonez_bulk_worker_processing_test LOGIN PASSWORD 'bulk_context_worker_ci_password' NOSUPERUSER NOBYPASSRLS INHERIT;
GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
DO $role_check$
BEGIN
  IF NOT pg_has_role('stonez_bulk_worker_processing_test','stonez_bulk_worker_processing','USAGE')
     OR pg_has_role('stonez_bulk_worker_processing_test','stonez_bulk_worker_processing','SET') THEN
    RAISE EXCEPTION 'processing login must inherit but must not SET ROLE into its policy group';
  END IF;
END
$role_check$;
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
  printf '{"iss":"stonez-bulk-import-worker","sub":"%s","aud":["stonez-postgres-bulk-import-context"],"jti":"%s","iat":%s,"nbf":%s,"exp":%s,"job_id":"%s","school_id":"%s","initiated_by":"%s","worker_id":"%s","lease_id":"%s"}' \
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
TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '66666666-6666-4666-8666-666666666666' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"

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
REPLAY="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '77777777-7777-4777-8777-777777777777' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$REPLAY" <<'SQL' >/dev/null
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
COMMIT;
SQL
expect_rejected "$REPLAY" "committed replay"
FORGED="$(printf '%s' "$TOKEN" | awk -F. '{ first=substr($3,1,1); replacement=(first=="A"?"B":"A"); print $1 "." $2 "." replacement substr($3,2) }')"
expect_rejected "$FORGED" "modified signature"
EXPIRED="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '88888888-8888-4888-8888-888888888888' "$((NOW-120))" "$((NOW-90))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$EXPIRED" "expired assertion"
OVERLONG="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '25252525-2525-4525-8525-252525252525' "$NOW" "$((NOW+61))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$OVERLONG" "assertion lifetime over sixty seconds"
NOT_YET_VALID="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '26262626-2626-4626-8626-262626262626' "$((NOW+30))" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$NOT_YET_VALID" "future issued-at/not-before claims"
UNKNOWN="$(make_token '{"alg":"HS256","kid":"unknown-key","typ":"JWT"}' "$(payload '99999999-9999-4999-8999-999999999999' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$UNKNOWN" "unknown key ID"
DUP_HEADER="$(make_token '{"alg":"HS256","kid":"ci-key-1","kid":"ci-key-1","typ":"JWT"}' "$(payload '22222222-2222-4222-8222-222222222222' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$DUP_HEADER" "duplicate JWS header member"
DUP_PAYLOAD="$(printf '{"iss":"stonez-bulk-import-worker","sub":"%s","aud":["stonez-postgres-bulk-import-context"],"jti":"23232323-2323-4323-8323-232323232323","iat":%s,"nbf":%s,"exp":%s,"job_id":"%s","school_id":"%s","school_id":"%s","initiated_by":"%s","worker_id":"%s","lease_id":"%s"}' "$WORKER_ID" "$NOW" "$NOW" "$((NOW+60))" "$JOB_ID" "$SCHOOL_ID" "$SCHOOL_ID" "$INITIATOR_ID" "$WORKER_ID" "$LEASE_ID")"
DUP_CLAIM="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$DUP_PAYLOAD" "$KEY_HEX")"
expect_rejected "$DUP_CLAIM" "duplicate JSON claim member"
ALG_NONE="$(make_token '{"alg":"none","kid":"ci-key-1","typ":"JWT"}' "$(payload '10101010-1010-4010-8010-101010101010' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$ALG_NONE" "algorithm substitution"
OVERLONG_TOKEN="$(printf 'A%.0s' {1..4097})"
expect_rejected "$OVERLONG_TOKEN" "oversized compact JWS input"
ALT_KEY_HEX="$(printf 'cd%.0s' {1..32})"
WRONG_KEY="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '11111111-1111-4111-8111-111111111112' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$ALT_KEY_HEX")"
expect_rejected "$WRONG_KEY" "untrusted signing key"
CROSS_TENANT="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '12121212-1212-4212-8212-121212121212' "$NOW" "$((NOW+60))" 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb' "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$CROSS_TENANT" "cross-tenant school claim"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "INSERT INTO public.students(id,school_id) VALUES ('24242424-2424-4424-8424-242424242424','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb')" >/dev/null 2>&1; then
  echo "Worker unexpectedly inserted a cross-tenant student" >&2
  exit 1
fi
echo "Rejected cross-tenant INSERT by worker RLS"

STALE_LEASE="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '13131313-1313-4313-8313-131313131313' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "UPDATE public.bulk_import_jobs SET lease_id='25252525-2525-4525-8525-252525252525' WHERE id='$JOB_ID'"
expect_rejected "$STALE_LEASE" "replaced lease generation"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "UPDATE public.bulk_import_jobs SET lease_id='$LEASE_ID', claimed_at=clock_timestamp() - interval '4 minutes 59 seconds' WHERE id='$JOB_ID'"

EXPIRING="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '24242424-2424-4424-8424-242424242424' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
(psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
BEGIN;
SELECT id FROM public.bulk_import_jobs WHERE id='$JOB_ID' FOR UPDATE;
SELECT pg_sleep(2);
COMMIT;
SQL
) &
lock_pid=$!
sleep 0.3
expect_rejected "$EXPIRING" "lease expiry during row-lock wait"
wait "$lock_pid"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "UPDATE public.bulk_import_jobs SET claimed_at=clock_timestamp() WHERE id='$JOB_ID'"

LOCK_NOW="$(date +%s)"
EXPIRING_ASSERTION="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '15151515-1515-4515-8515-151515151515' "$LOCK_NOW" "$((LOCK_NOW+1))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
(psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
BEGIN;
SELECT id FROM public.bulk_import_jobs WHERE id='$JOB_ID' FOR UPDATE;
SELECT pg_sleep(7);
COMMIT;
SQL
) &
assertion_lock_pid=$!
sleep 0.3
expect_rejected "$EXPIRING_ASSERTION" "assertion expiry during queue row-lock wait"
wait "$assertion_lock_pid"

CONCURRENT="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '14141414-1414-4414-8414-141414141414' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
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

echo "== Verify private key custody and GUC-independent access =="
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT key_bytes FROM stonez_context_private.verification_keys" >/dev/null 2>&1; then
  echo "Worker unexpectedly read verifier key material" >&2
  exit 1
fi
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT * FROM stonez_context_private.verified_contexts" >/dev/null 2>&1; then
  echo "Worker unexpectedly read verified context rows" >&2
  exit 1
fi
psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT id,school_id,email,role FROM public.users LIMIT 0" >/dev/null
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT password_hash FROM public.users" >/dev/null 2>&1; then
  echo "Worker unexpectedly read user password hashes" >&2
  exit 1
fi
no_context_rows="$(psql "$worker_url" -X -qAt -v ON_ERROR_STOP=1 -c "BEGIN; SELECT set_config('app.current_school_id','$SCHOOL_ID',true); SELECT set_config('app.current_user_id','$INITIATOR_ID',true); SELECT count(*) FROM public.students; COMMIT;" | tail -n 1)"
if [[ "$no_context_rows" != "0" ]]; then
  echo "A raw tenant GUC unexpectedly granted worker row access: $no_context_rows" >&2
  exit 1
fi

echo "== Verify key overlap, retirement, emergency revocation, and audit trail =="
KEY2_HEX="$(printf 'cd%.0s' {1..32})"
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
SELECT stonez_context_private.register_verification_key(
  'ci-key-2', decode(repeat('cd',32),'hex'), clock_timestamp() - interval '5 minutes', NULL
);
SQL
KEY2_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-2","typ":"JWT"}' "$(payload '19191919-1919-4919-8919-191919191919' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY2_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$KEY2_TOKEN" <<'SQL' >/dev/null
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
COMMIT;
SQL
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "SELECT stonez_context_private.retire_verification_key('ci-key-1', clock_timestamp() - interval '1 second')"
OLD_KEY_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '20202020-2020-4020-8020-202020202020' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
expect_rejected "$OLD_KEY_TOKEN" "retired verification key"
KEY2_AFTER_RETIRE="$(make_token '{"alg":"HS256","kid":"ci-key-2","typ":"JWT"}' "$(payload '21212121-2121-4121-8121-212121212121' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY2_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$KEY2_AFTER_RETIRE" <<'SQL' >/dev/null
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
COMMIT;
SQL
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "SELECT stonez_context_private.revoke_verification_key('ci-key-2')"
expect_rejected "$KEY2_AFTER_RETIRE" "emergency-revoked verification key"
audit_events="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM stonez_context_private.key_management_events")"
if [[ "$audit_events" != "4" ]]; then
  echo "Expected four key-management audit events, got '$audit_events'" >&2
  exit 1
fi

echo "PostgreSQL signed-context verifier adversarial tests passed."
