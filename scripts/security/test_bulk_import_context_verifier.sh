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
    kind text NOT NULL DEFAULT 'students', file_name text NOT NULL DEFAULT 'fixture.csv',
    send_credentials boolean NOT NULL DEFAULT false, rows_json text NOT NULL DEFAULT '[]',
    attempts integer NOT NULL DEFAULT 0, processed_rows_json jsonb NOT NULL DEFAULT '[]'::jsonb, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    total integer NOT NULL DEFAULT 0, valid integer NOT NULL DEFAULT 0,
    created integer NOT NULL DEFAULT 0, updated integer NOT NULL DEFAULT 0,
    skipped integer NOT NULL DEFAULT 0, failed integer NOT NULL DEFAULT 0,
    credentials_sent integer NOT NULL DEFAULT 0, credential_emails_failed integer NOT NULL DEFAULT 0,
    errors_json jsonb NOT NULL DEFAULT '[]'::jsonb, last_error text NOT NULL DEFAULT ''
);
INSERT INTO public.bulk_import_jobs(id,school_id,initiated_by,worker_id,lease_id,status,claimed_at,send_credentials)
VALUES ('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222',
'33333333-3333-4333-8333-333333333333','44444444-4444-4444-8444-444444444444',
'55555555-5555-4555-8555-555555555555','running',clock_timestamp(),true),
('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','22222222-2222-4222-8222-222222222222',
'33333333-3333-4333-8333-333333333333','dddddddd-dddd-4ddd-8ddd-dddddddddddd',
'eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee','running',clock_timestamp(),false);
CREATE TABLE public.users(id uuid PRIMARY KEY, school_id uuid, name text, email text, role text, password_hash text, active boolean NOT NULL DEFAULT true);
CREATE TABLE public.students(id uuid PRIMARY KEY, school_id uuid NOT NULL, user_id uuid, admission_number text, date_of_birth date, gender text, guardian_name text, guardian_phone text, guardian_email text, enrollment_status text NOT NULL DEFAULT 'active', created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp());
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

# Exercise installer preflight against deliberately unsafe pre-existing roles.
# Each failed installer run is transactional; no verifier objects are left behind.
expect_role_preflight_rejection() {
  local label="$1"
  if psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_verifier.sql >/dev/null 2>&1; then
    echo "Verifier installer accepted unsafe pre-existing role state: $label" >&2
    exit 1
  fi
  echo "Rejected unsafe pre-existing role state: $label"
}
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE stonez_bulk_context_owner NOLOGIN NOSUPERUSER NOBYPASSRLS INHERIT;
SQL
expect_role_preflight_rejection "context owner INHERIT"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "DROP ROLE stonez_bulk_context_owner"

psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE stonez_bulk_context_key_admin NOLOGIN NOSUPERUSER NOBYPASSRLS INHERIT;
SQL
expect_role_preflight_rejection "key-admin INHERIT"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "DROP ROLE stonez_bulk_context_key_admin"

for protected_role in stonez_bulk_context_owner stonez_bulk_context_key_admin; do
  # A protected role must not inherit another role.
  psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE $protected_role NOLOGIN NOSUPERUSER NOBYPASSRLS NOINHERIT;
CREATE ROLE verifier_preflight_parent NOLOGIN;
GRANT verifier_preflight_parent TO $protected_role;
SQL
  expect_role_preflight_rejection "$protected_role inherits a parent role"
  psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
REVOKE verifier_preflight_parent FROM $protected_role;
DROP ROLE verifier_preflight_parent;
DROP ROLE $protected_role;
SQL

  # No unrelated login/group may receive a protected owner/admin role.
  psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE $protected_role NOLOGIN NOSUPERUSER NOBYPASSRLS NOINHERIT;
CREATE ROLE verifier_preflight_member NOLOGIN;
GRANT $protected_role TO verifier_preflight_member;
SQL
  expect_role_preflight_rejection "$protected_role granted to an unrelated role"
  psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
REVOKE $protected_role FROM verifier_preflight_member;
DROP ROLE verifier_preflight_member;
DROP ROLE $protected_role;
SQL
done

psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_verifier.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_context_worker_policies.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_account_setup_outbox.sql
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
DO $outbox_privileges$
DECLARE
  v_owner oid := (SELECT oid FROM pg_roles WHERE rolname='stonez_bulk_context_owner');
  v_processing oid := (SELECT oid FROM pg_roles WHERE rolname='stonez_bulk_worker_processing');
  v_table oid := 'stonez_context_private.account_setup_outbox'::regclass;
  v_functions oid[] := ARRAY[
    'stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text)'::regprocedure::oid,
    'stonez_context_private.claim_account_setup_notification()'::regprocedure::oid,
    'stonez_context_private.complete_account_setup_notification(uuid,uuid,text)'::regprocedure::oid,
    'stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer)'::regprocedure::oid,
    'stonez_context_private.account_setup_notification_counts()'::regprocedure::oid
  ];
BEGIN
  IF cardinality(v_functions) <> 5 OR EXISTS (
      SELECT 1 FROM unnest(v_functions) AS functions(function_oid)
      JOIN pg_proc AS procedure ON procedure.oid=functions.function_oid
      WHERE procedure.prosecdef IS DISTINCT FROM true
         OR procedure.proowner IS DISTINCT FROM v_owner
         OR NOT ('search_path=pg_catalog'=ANY(COALESCE(procedure.proconfig,ARRAY[]::text[])))
  ) THEN
    RAISE EXCEPTION 'all outbox entry points must be SECURITY DEFINER functions owned by the context owner with a fixed search_path';
  END IF;
  IF (SELECT relowner FROM pg_class WHERE oid=v_table) IS DISTINCT FROM v_owner THEN
    RAISE EXCEPTION 'outbox table must be owned by the dedicated context owner';
  END IF;
  IF EXISTS (
      SELECT 1 FROM pg_attribute
       WHERE attrelid=v_table AND attnum>0 AND NOT attisdropped
         AND attname IN ('password','password_hash','temporary_password','reset_token','token','signed_assertion')
  ) THEN
    RAISE EXCEPTION 'outbox schema must never contain password or bearer-token material';
  END IF;
  IF has_table_privilege(v_processing,v_table,'SELECT')
     OR has_table_privilege(v_processing,v_table,'INSERT')
     OR has_table_privilege(v_processing,v_table,'UPDATE')
     OR has_table_privilege(v_processing,v_table,'DELETE') THEN
    RAISE EXCEPTION 'processing group has direct outbox table privileges';
  END IF;
  IF NOT has_function_privilege(v_processing,'stonez_context_private.enqueue_account_setup_notification(uuid,text,text,text)','EXECUTE')
     OR NOT has_function_privilege(v_processing,'stonez_context_private.claim_account_setup_notification()','EXECUTE')
     OR NOT has_function_privilege(v_processing,'stonez_context_private.complete_account_setup_notification(uuid,uuid,text)','EXECUTE')
     OR NOT has_function_privilege(v_processing,'stonez_context_private.retry_account_setup_notification(uuid,uuid,text,integer)','EXECUTE')
     OR NOT has_function_privilege(v_processing,'stonez_context_private.account_setup_notification_counts()','EXECUTE') THEN
    RAISE EXCEPTION 'processing group is missing a scoped outbox function grant';
  END IF;
  IF EXISTS (
      SELECT 1 FROM pg_proc AS procedure
      CROSS JOIN LATERAL aclexplode(COALESCE(procedure.proacl,acldefault('f',procedure.proowner))) AS privilege
      WHERE procedure.oid=ANY(v_functions)
        AND privilege.grantee=0 AND privilege.privilege_type='EXECUTE'
  ) THEN
    RAISE EXCEPTION 'PUBLIC can execute an outbox entry point';
  END IF;
END
$outbox_privileges$;
SQL
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

echo "== Verify least-privilege user insertion and role guard =="
USER_INSERT_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '27272727-2727-4727-8727-272727272727' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$USER_INSERT_TOKEN" <<'SQL'
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
INSERT INTO public.users(id,school_id,name,email,password_hash,role,active)
VALUES ('28282828-2828-4828-8828-282828282828',
        '22222222-2222-4222-8222-222222222222','Imported Student',
        'imported-student@example.test','not-a-real-hash','student',true);
COMMIT;
SQL
PRIVILEGED_USER_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-key-1","typ":"JWT"}' "$(payload '29292929-2929-4929-8929-292929292929' "$NOW" "$((NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$KEY_HEX")"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$PRIVILEGED_USER_TOKEN" <<'SQL' >/dev/null 2>&1
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
INSERT INTO public.users(id,school_id,name,email,password_hash,role,active)
VALUES ('30303030-3030-4030-8030-303030303030',
        '22222222-2222-4222-8222-222222222222','Injected Administrator',
        'injected-admin@example.test','not-a-real-hash','super_admin',true);
COMMIT;
SQL
then
  echo "Bulk worker unexpectedly inserted a super_admin account" >&2
  exit 1
fi
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "INSERT INTO public.users(id,school_id,name,email,password_hash,role,active) VALUES ('31313131-3131-4131-8131-313131313131','$SCHOOL_ID','No context','no-context@example.test','x','student',true)" >/dev/null 2>&1; then
  echo "Bulk worker unexpectedly inserted a user without verified context" >&2
  exit 1
fi
echo "Accepted student import; rejected super_admin and context-free inserts"

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
pruned_contexts="$(psql "$runner_url" -X -qAt -v ON_ERROR_STOP=1 -c "BEGIN; SET LOCAL ROLE stonez_bulk_context_key_admin; SELECT stonez_context_private.prune_expired_contexts(); COMMIT;")"
if [[ "$pruned_contexts" != "0" ]]; then
  echo "Expected no fresh verifier contexts to be pruned, got '$pruned_contexts'" >&2
  exit 1
fi

echo "PostgreSQL signed-context verifier adversarial tests passed."


echo "== Private durable account-setup notification outbox =="
# The installer is intentionally exercised only against this disposable PG17 fixture.
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_account_setup_outbox.sql
OUTBOX_KEY_HEX="${BULK_CONTEXT_TEST_SIGNING_KEY_HEX:-$(printf 'ef%.0s' {1..32})}"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "SELECT stonez_context_private.register_verification_key('ci-outbox-key', decode('$OUTBOX_KEY_HEX','hex'), clock_timestamp()-interval '5 minutes', NULL)"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT * FROM stonez_context_private.account_setup_outbox" >/dev/null 2>&1; then
  echo "Processing login unexpectedly read private outbox rows directly" >&2
  exit 1
fi
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "INSERT INTO stonez_context_private.account_setup_outbox(school_id,job_id,user_id,recipient_email,account_role) VALUES ('$SCHOOL_ID','$JOB_ID','34343434-3434-4434-8434-343434343434','direct@example.test','student')" >/dev/null 2>&1; then
  echo "Processing login unexpectedly inserted directly into private outbox" >&2
  exit 1
fi
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT stonez_context_private.enqueue_account_setup_notification('34343434-3434-4434-8434-343434343434','direct@example.test','No Context','student')" >/dev/null 2>&1; then
  echo "Outbox enqueue unexpectedly succeeded without a verified transaction context" >&2
  exit 1
fi

OUTBOX_NOW="$(date +%s)"
OUTBOX_USER_ID="34343434-3434-4434-8434-343434343434"
OUTBOX_JTI="35353535-3535-4535-8535-353535353535"
OUTBOX_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$OUTBOX_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
# Account creation and enqueue are one transaction: rollback removes both.
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$OUTBOX_TOKEN" -v user_id="$OUTBOX_USER_ID" <<'SQL'
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
INSERT INTO public.users(id,school_id,name,email,password_hash,role,active)
VALUES (:'user_id','22222222-2222-4222-8222-222222222222','Outbox Student',
        'outbox-student@example.test','non-secret-test-hash','student',true);
SELECT stonez_context_private.enqueue_account_setup_notification(
    :'user_id','outbox-student@example.test','Outbox Student','student'
);
ROLLBACK;
SQL
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT 1 FROM public.users WHERE id='$OUTBOX_USER_ID'" | grep -q 1; then
  echo "Account row survived transaction rollback" >&2
  exit 1
fi

# Reuse the rolled-back assertion: both the user and outbox event now commit.
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$OUTBOX_TOKEN" -v user_id="$OUTBOX_USER_ID" <<'SQL'
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
INSERT INTO public.users(id,school_id,name,email,password_hash,role,active)
VALUES (:'user_id','22222222-2222-4222-8222-222222222222','Outbox Student',
        'outbox-student@example.test','non-secret-test-hash','student',true);
SELECT stonez_context_private.enqueue_account_setup_notification(
    :'user_id','outbox-student@example.test','Outbox Student','student'
);
COMMIT;
SQL

# A restarted worker (fresh assertion ID, same active queue lease) can claim
# the durable event after the account-creation transaction has committed.
CLAIM_JTI="36363636-3636-4636-8636-363636363636"
CLAIM_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$CLAIM_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
CLAIM_ROW="$(psql "$worker_url" -X -A -t -F '|' -v ON_ERROR_STOP=1 -v token="$CLAIM_TOKEN" -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$CLAIM_TOKEN'); SELECT event_id,user_id,recipient_email,delivery_lease,attempt_count FROM stonez_context_private.claim_account_setup_notification(); COMMIT;" | awk -F'|' 'NF==5 {row=$0} END {print row}')"
IFS='|' read -r EVENT_ID CLAIMED_USER CLAIMED_EMAIL OLD_DELIVERY_LEASE ATTEMPT_COUNT <<< "$CLAIM_ROW"
if [[ -z "$EVENT_ID" || "$CLAIMED_USER" != "$OUTBOX_USER_ID" || "$CLAIMED_EMAIL" != "outbox-student@example.test" || "$ATTEMPT_COUNT" != "1" ]]; then
  echo "Durable outbox event was not claimable after worker restart: $CLAIM_ROW" >&2
  exit 1
fi

# Expire the first lease using the fixture owner, reclaim, and prove the old
# lease generation cannot complete the newly claimed event.
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "UPDATE stonez_context_private.account_setup_outbox SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE event_id='$EVENT_ID'"
RECLAIM_JTI="37373737-3737-4737-8737-373737373737"
RECLAIM_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$RECLAIM_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
RECLAIM_ROW="$(psql "$worker_url" -X -A -t -F '|' -v ON_ERROR_STOP=1 -v token="$RECLAIM_TOKEN" -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$RECLAIM_TOKEN'); SELECT event_id,delivery_lease,attempt_count FROM stonez_context_private.claim_account_setup_notification(); COMMIT;" | awk -F'|' 'NF==3 {row=$0} END {print row}')"
IFS='|' read -r RECLAIMED_EVENT NEW_DELIVERY_LEASE NEW_ATTEMPTS <<< "$RECLAIM_ROW"
if [[ "$RECLAIMED_EVENT" != "$EVENT_ID" || "$NEW_DELIVERY_LEASE" == "$OLD_DELIVERY_LEASE" || "$NEW_ATTEMPTS" != "2" ]]; then
  echo "Expired lease was not safely reclaimed: $RECLAIM_ROW" >&2
  exit 1
fi
UNSAFE_RETRY_JTI="41414141-4141-4141-8141-414141414141"
UNSAFE_RETRY_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$UNSAFE_RETRY_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$UNSAFE_RETRY_TOKEN" -v event_id="$EVENT_ID" -v lease="$NEW_DELIVERY_LEASE" <<'SQL' >/dev/null 2>&1
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
SELECT stonez_context_private.retry_account_setup_notification(:'event_id', :'lease', 'Bearer secret-value', 30);
COMMIT;
SQL
then
  echo "Outbox retry accepted arbitrary error text that could contain secrets" >&2
  exit 1
fi
STALE_COMPLETE_JTI="38383838-3838-4838-8838-383838383838"
STALE_COMPLETE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$STALE_COMPLETE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
stale_result="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -v token="$STALE_COMPLETE_TOKEN" -v event_id="$EVENT_ID" -v old_lease="$OLD_DELIVERY_LEASE" -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$STALE_COMPLETE_TOKEN'); SELECT stonez_context_private.complete_account_setup_notification('$EVENT_ID', '$OLD_DELIVERY_LEASE', 'stale-provider-id'); COMMIT;" | tail -n 1)"
if [[ "$stale_result" != "f" ]]; then
  echo "Stale delivery lease unexpectedly completed the event: $stale_result" >&2
  exit 1
fi
COMPLETE_JTI="39393939-3939-4939-8939-393939393939"
COMPLETE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$COMPLETE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
complete_result="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -v token="$COMPLETE_TOKEN" -v event_id="$EVENT_ID" -v lease="$NEW_DELIVERY_LEASE" -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$COMPLETE_TOKEN'); SELECT stonez_context_private.complete_account_setup_notification('$EVENT_ID', '$NEW_DELIVERY_LEASE', 'provider-test-123'); COMMIT;" | tail -n 1)"
if [[ "$complete_result" != "t" ]]; then
  echo "Current delivery lease could not complete event: $complete_result" >&2
  exit 1
fi

# Idempotent enqueue returns the same event for the same job/user and does not
# create duplicate notifications on a retried account row.
DUPLICATE_JTI="40404040-4040-4040-8040-404040404040"
DUPLICATE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$DUPLICATE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
duplicate_id="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -v token="$DUPLICATE_TOKEN" -v user_id="$OUTBOX_USER_ID" -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$DUPLICATE_TOKEN'); SELECT stonez_context_private.enqueue_account_setup_notification('$OUTBOX_USER_ID','outbox-student@example.test','Outbox Student','student'); COMMIT;" | tail -n 1)"
if [[ "$duplicate_id" != "$EVENT_ID" ]]; then
  echo "Idempotent enqueue returned a different event ID: $duplicate_id" >&2
  exit 1
fi
CONCURRENT_USER_ID="42424242-4242-4242-8242-424242424242"
CONCURRENT_ENQUEUE_JTI="43434343-4343-4343-8343-434343434343"
CONCURRENT_ENQUEUE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$CONCURRENT_ENQUEUE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$CONCURRENT_ENQUEUE_TOKEN" -v user_id="$CONCURRENT_USER_ID" <<'SQL'
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
INSERT INTO public.users(id,school_id,name,email,password_hash,role,active)
VALUES (:'user_id','22222222-2222-4222-8222-222222222222','Concurrent Student',
        'concurrent-outbox@example.test','non-secret-test-hash','student',true);
SELECT stonez_context_private.enqueue_account_setup_notification(
    :'user_id','concurrent-outbox@example.test','Concurrent Student','student'
);
COMMIT;
SQL
CONCURRENT_FIRST_JTI="44444444-4444-4444-8444-444444444444"
CONCURRENT_SECOND_JTI="45454545-4545-4545-8454-454545454545"
CONCURRENT_FIRST_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$CONCURRENT_FIRST_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
CONCURRENT_SECOND_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$CONCURRENT_SECOND_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
concurrent_output="$(mktemp)"
psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$CONCURRENT_FIRST_TOKEN'); SELECT event_id FROM stonez_context_private.claim_account_setup_notification(); SELECT pg_sleep(3); COMMIT;" >"$concurrent_output" &
concurrent_pid=$!
sleep 1
second_claim="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$CONCURRENT_SECOND_TOKEN'); SELECT event_id FROM stonez_context_private.claim_account_setup_notification(); COMMIT;")"
wait "$concurrent_pid"
first_claim="$(grep -E '^[0-9a-f-]{36}$' "$concurrent_output" | head -n 1)"
rm -f "$concurrent_output"
if [[ -z "$first_claim" || -n "$second_claim" ]]; then
  echo "Concurrent outbox claims were not mutually exclusive: first=$first_claim second=$second_claim" >&2
  exit 1
fi


# Direct mutation is denied as well as direct reads/inserts.
for operation in \
  "UPDATE stonez_context_private.account_setup_outbox SET status='delivered'" \
  "DELETE FROM stonez_context_private.account_setup_outbox"
do
  if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "$operation" >/dev/null 2>&1; then
    echo "Processing login unexpectedly performed direct outbox mutation: $operation" >&2
    exit 1
  fi
done
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -c "SELECT * FROM stonez_context_private.claim_account_setup_notification()" >/dev/null 2>&1; then
  echo "Outbox claim unexpectedly succeeded without verified context" >&2
  exit 1
fi

# An unrelated role has schema visibility but receives no EXECUTE privilege.
UNRELATED_ROLE="stonez_outbox_unrelated_test"
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE $UNRELATED_ROLE NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA stonez_context_private TO $UNRELATED_ROLE;
SQL
if psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "SET ROLE $UNRELATED_ROLE; SELECT stonez_context_private.claim_account_setup_notification()" >/dev/null 2>&1; then
  echo "Unrelated role unexpectedly executed a private outbox function" >&2
  exit 1
fi
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "DROP OWNED BY $UNRELATED_ROLE; DROP ROLE $UNRELATED_ROLE"

# A failed enqueue must abort the account transaction, not leave an account
# without its durable setup notification.
FAILED_ENQUEUE_USER_ID="46464646-4646-4646-8464-464646464646"
FAILED_ENQUEUE_JTI="47474747-4747-4747-8474-474747474747"
FAILED_ENQUEUE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$FAILED_ENQUEUE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$FAILED_ENQUEUE_TOKEN" -v user_id="$FAILED_ENQUEUE_USER_ID" <<'SQL' >/dev/null 2>&1
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
INSERT INTO public.users(id,school_id,name,email,password_hash,role,active)
VALUES (:'user_id','22222222-2222-4222-8222-222222222222','Failed Enqueue',
        'failed-enqueue@example.test','non-secret-test-hash','student',true);
SELECT stonez_context_private.enqueue_account_setup_notification(
    :'user_id','different-address@example.test','Failed Enqueue','student'
);
COMMIT;
SQL
then
  echo "Mismatched outbox enqueue unexpectedly committed" >&2
  exit 1
fi
failed_enqueue_user_count="$(psql "$runner_url" -X -qAt -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM public.users WHERE id='$FAILED_ENQUEUE_USER_ID'")"
if [[ "$failed_enqueue_user_count" != "0" ]]; then
  echo "Account survived a failed atomic outbox enqueue" >&2
  exit 1
fi

# The SECURITY DEFINER identity lookup is itself tenant-scoped by RLS.
FOREIGN_USER_ID="96969696-9696-4969-8969-969696969696"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "INSERT INTO public.users(id,school_id,name,email,password_hash,role,active) VALUES ('$FOREIGN_USER_ID','99999999-9999-4999-8999-999999999999','Foreign Student','foreign-user@example.test','non-secret-test-hash','student',true)"
FOREIGN_ENQUEUE_JTI="95959595-9595-4959-8959-959595959595"
FOREIGN_ENQUEUE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$FOREIGN_ENQUEUE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
if psql "$worker_url" -X -v ON_ERROR_STOP=1 -v token="$FOREIGN_ENQUEUE_TOKEN" -v user_id="$FOREIGN_USER_ID" <<'SQL' >/dev/null 2>&1
BEGIN;
SELECT stonez_context_private.verify_bulk_import_context(:'token');
SELECT stonez_context_private.enqueue_account_setup_notification(
    :'user_id','foreign-user@example.test','Foreign Student','student'
);
COMMIT;
SQL
then
  echo "Verified school context unexpectedly enqueued a different school's user" >&2
  exit 1
fi

# Claims and counts are bound to both the verified school and exact job.
FOREIGN_EVENT_ID="48484848-4848-4848-8484-484848484848"
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "INSERT INTO stonez_context_private.account_setup_outbox(event_id,school_id,job_id,user_id,recipient_email,account_role) VALUES ('$FOREIGN_EVENT_ID','99999999-9999-4999-8999-999999999999','98989898-9898-4989-8989-989898989898','97979797-9797-4979-8979-979797979797','foreign@example.test','student')"
SCOPE_JTI="49494949-4949-4949-8949-494949494949"
SCOPE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$SCOPE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
scoped_counts="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$SCOPE_TOKEN'); SELECT delivered_count||'|'||failed_count||'|'||pending_count FROM stonez_context_private.account_setup_notification_counts(); COMMIT;" | awk -F'|' 'NF==3 {value=$0} END {print value}')"
# One same-job event is intentionally still delivering from the preceding
# concurrent-claim test. The foreign school's pending event must not add to it.
if [[ "$scoped_counts" != "1|0|1" ]]; then
  echo "Outbox counters leaked another job or school's events: $scoped_counts" >&2
  exit 1
fi
SCOPE_CLAIM_JTI="49494949-4949-4949-8949-494949494950"
SCOPE_CLAIM_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$SCOPE_CLAIM_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
scope_claim="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$SCOPE_CLAIM_TOKEN'); SELECT event_id FROM stonez_context_private.claim_account_setup_notification(); COMMIT;" | grep -E '^[0-9a-f-]{36}$' || true)"
if [[ -n "$scope_claim" ]]; then
  echo "Current job claimed an event belonging to another school/job: $scope_claim" >&2
  exit 1
fi
FOREIGN_COMPLETE_JTI="50505050-5050-4050-8050-505050505050"
FOREIGN_COMPLETE_TOKEN="$(make_token '{"alg":"HS256","kid":"ci-outbox-key","typ":"JWT"}' "$(payload "$FOREIGN_COMPLETE_JTI" "$OUTBOX_NOW" "$((OUTBOX_NOW+60))" "$SCHOOL_ID" "$LEASE_ID")" "$OUTBOX_KEY_HEX")"
foreign_complete="$(psql "$worker_url" -X -q -A -t -v ON_ERROR_STOP=1 -c "BEGIN; SELECT stonez_context_private.verify_bulk_import_context('$FOREIGN_COMPLETE_TOKEN'); SELECT stonez_context_private.complete_account_setup_notification('$FOREIGN_EVENT_ID','51515151-5151-4151-8151-515151515151',NULL); COMMIT;" | tail -n 1)"
if [[ "$foreign_complete" != "f" ]]; then
  echo "Current job completed an event belonging to another school/job: $foreign_complete" >&2
  exit 1
fi
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "DELETE FROM stonez_context_private.account_setup_outbox WHERE event_id='$FOREIGN_EVENT_ID'"

echo "Private signed-context credential outbox permission, atomicity, restart, lease-fencing, completion, concurrency, and idempotency tests passed."
