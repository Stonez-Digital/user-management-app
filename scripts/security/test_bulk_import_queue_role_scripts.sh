#!/usr/bin/env bash
set -euo pipefail

: "${BULK_QUEUE_SCRIPT_TEST_DATABASE_URL:?must point to the disposable PostgreSQL CI database}"
: "${BULK_QUEUE_SCRIPT_TEST_RUNTIME_URL:?must point to the disposable restricted runtime login}"
: "${BULK_QUEUE_SCRIPT_TEST_OUTSIDER_URL:?must point to the disposable unrelated login}"
: "${BULK_QUEUE_SCRIPT_TEST_DATABASE:?must explicitly name the disposable database}"

runner_url="$BULK_QUEUE_SCRIPT_TEST_DATABASE_URL"
runtime_url="$BULK_QUEUE_SCRIPT_TEST_RUNTIME_URL"
outsider_url="$BULK_QUEUE_SCRIPT_TEST_OUTSIDER_URL"

actual_db="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c 'SELECT current_database()')"
is_superuser="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT rolsuper FROM pg_roles WHERE rolname = current_user")"
can_create_role="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT rolcreaterole FROM pg_roles WHERE rolname = current_user")"

if [[ "$actual_db" != "$BULK_QUEUE_SCRIPT_TEST_DATABASE" ]]; then
  echo "Refusing SQL fixture: connected to '$actual_db', expected '$BULK_QUEUE_SCRIPT_TEST_DATABASE'" >&2
  exit 1
fi
if [[ "$is_superuser" != "f" || "$can_create_role" != "t" ]]; then
  echo "Refusing SQL fixture: runner must be a non-superuser with CREATEROLE" >&2
  exit 1
fi
if [[ "$actual_db" == *prod* || "$actual_db" == *production* ]]; then
  echo "Refusing SQL fixture against production-looking database '$actual_db'" >&2
  exit 1
fi

# This fixture runs only in a disposable CI database. The database owner gives
# the non-superuser runner ownership of public so the operator scripts can
# exercise the same schema grants they need in the managed staging database.
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE public.bulk_import_jobs (
    id uuid PRIMARY KEY,
    school_id uuid NOT NULL,
    initiated_by uuid NOT NULL,
    kind text NOT NULL,
    file_name text NOT NULL,
    status text NOT NULL,
    total integer NOT NULL DEFAULT 0,
    valid integer NOT NULL DEFAULT 0,
    created integer NOT NULL DEFAULT 0,
    updated integer NOT NULL DEFAULT 0,
    skipped integer NOT NULL DEFAULT 0,
    failed integer NOT NULL DEFAULT 0,
    send_credentials boolean NOT NULL DEFAULT false,
    credentials_sent integer NOT NULL DEFAULT 0,
    credential_emails_failed integer NOT NULL DEFAULT 0,
    worker_id uuid,
    claimed_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    rows_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    errors_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
SQL

echo "== Provision roles =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/provision_bulk_import_queue_roles.sql

echo "== Verify provisioned roles and grants =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/verify_bulk_import_queue_roles.sql

echo "== Install actual restricted claim function =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_claim_function.sql

echo "== Create restricted runtime and unrelated logins, then seed one job =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE bq_script_runtime LOGIN PASSWORD 'queue_script_runtime_ci_password' NOSUPERUSER NOBYPASSRLS;
GRANT stonez_api_runtime TO bq_script_runtime WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
CREATE ROLE bq_script_outsider LOGIN PASSWORD 'queue_script_outsider_ci_password' NOSUPERUSER NOBYPASSRLS;
INSERT INTO public.bulk_import_jobs (
    id, school_id, initiated_by, kind, file_name, status, rows_json, errors_json
) VALUES (
    '11111111-1111-4111-8111-111111111111',
    '22222222-2222-4222-8222-222222222222',
    '33333333-3333-4333-8333-333333333333',
    'students',
    'fixture.csv',
    'queued',
    '[{"name":"Fixture student"}]'::jsonb,
    '[]'::jsonb
);
SQL

echo "== Assert runtime cannot see queue rows directly without tenant context =="
direct_rows="$(psql "$runtime_url" -X -A -t -v ON_ERROR_STOP=1 -c 'SELECT count(*) FROM public.bulk_import_jobs')"
if [[ "$direct_rows" != "0" ]]; then
  echo "Runtime unexpectedly read $direct_rows queue rows without tenant context" >&2
  exit 1
fi

echo "== Assert runtime can execute the installed SECURITY DEFINER claim function =="
claimed_rows="$(psql "$runtime_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM stonez_internal.claim_next_bulk_import_job('44444444-4444-4444-8444-444444444444')")"
if [[ "$claimed_rows" != "1" ]]; then
  echo "Expected the actual installed claim function to return one job; got '$claimed_rows'" >&2
  exit 1
fi

echo "== Assert privileged CREATEROLE installer cannot invoke the function =="
if psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "SELECT * FROM stonez_internal.claim_next_bulk_import_job('66666666-6666-4666-8666-666666666666')" >/dev/null 2>&1; then
  echo "Privileged CREATEROLE installer unexpectedly executed the claim function" >&2
  exit 1
fi

claim_state="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT status || ':' || attempts::text FROM public.bulk_import_jobs WHERE id = '11111111-1111-4111-8111-111111111111'")"
if [[ "$claim_state" != "running:1" ]]; then
  echo "Unexpected persisted claim state '$claim_state'" >&2
  exit 1
fi

echo "== Assert unrelated login cannot execute the installed function =="
if psql "$outsider_url" -X -v ON_ERROR_STOP=1 -c "SELECT * FROM stonez_internal.claim_next_bulk_import_job('55555555-5555-4555-8555-555555555555')" >/dev/null 2>&1; then
  echo "Unrelated login unexpectedly executed the restricted claim function" >&2
  exit 1
fi

echo "Bulk-import queue SQL script fixture passed."
