#!/usr/bin/env bash
set -euo pipefail

: "${BULK_QUEUE_SCRIPT_TEST_DATABASE_URL:?must point to the disposable PostgreSQL CI database}"
: "${BULK_QUEUE_SCRIPT_TEST_CLAIM_URL:?must point to the disposable restricted runtime login}"
: "${BULK_QUEUE_SCRIPT_TEST_OUTSIDER_URL:?must point to the disposable unrelated login}"
: "${BULK_QUEUE_SCRIPT_TEST_DATABASE:?must explicitly name the disposable database}"

runner_url="$BULK_QUEUE_SCRIPT_TEST_DATABASE_URL"
claim_url="$BULK_QUEUE_SCRIPT_TEST_CLAIM_URL"
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

# The operator script must fail before creating roles or grants when the
# application lease-generation migration has not added bulk_import_jobs.lease_id.
preflight_output="$(mktemp)"
if psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/provision_bulk_import_queue_roles.sql >"$preflight_output" 2>&1; then
  echo "Queue-role provisioning unexpectedly accepted a schema without lease_id" >&2
  rm -f "$preflight_output"
  exit 1
fi
if ! grep -q "bulk_import_jobs.lease_id is missing; apply application migration 48 before queue-role provisioning" "$preflight_output"; then
  cat "$preflight_output" >&2
  echo "Queue-role provisioning failed for an unexpected reason" >&2
  rm -f "$preflight_output"
  exit 1
fi
rm -f "$preflight_output"

# Simulate application migration 48 before running the operator role scripts.
psql "$runner_url" -X -v ON_ERROR_STOP=1 -c "ALTER TABLE public.bulk_import_jobs ADD COLUMN lease_id uuid"

echo "== Provision roles after lease-generation migration =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/provision_bulk_import_queue_roles.sql

echo "== Verify provisioned roles and grants =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/verify_bulk_import_queue_roles.sql

echo "== Install actual restricted claim function =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/install_bulk_import_claim_function.sql

echo "== Seed a table available to API runtime but not the claim credential =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE public.billing_plans (id integer PRIMARY KEY);
GRANT SELECT ON TABLE public.billing_plans TO stonez_api_runtime;
SQL

echo "== Provision dedicated queue-claim login using the interactive operator script =="
printf '%s\n%s\n' 'queue_script_worker_ci_password' 'queue_script_worker_ci_password' | psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/provision_bulk_import_worker_login.psql

echo "== Create unrelated login and seed one job =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
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

echo "== Simulate legacy API membership and transition the existing worker login =="
psql "$runner_url" -X -v ON_ERROR_STOP=1 <<'SQL'
REVOKE stonez_bulk_queue_claim FROM stonez_bulk_worker;
GRANT stonez_api_runtime TO stonez_bulk_worker
    WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
SQL
psql "$runner_url" -X -v ON_ERROR_STOP=1 -f scripts/security/migrate_bulk_import_worker_to_queue_claim.psql

echo "== Assert claim login inherits only the dedicated queue-claim group =="
claim_membership="$(psql "$claim_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT pg_has_role(session_user, 'stonez_bulk_queue_claim', 'USAGE')::text || ':' || pg_has_role(session_user, 'stonez_bulk_queue_claim', 'SET')::text || ':' || pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER')::text || ':' || pg_has_role(session_user, 'stonez_bulk_queue_owner', 'MEMBER')::text")"
if [[ "$claim_membership" != "true:false:false:false" ]]; then
  echo "Unexpected queue-claim login membership state '$claim_membership'" >&2
  exit 1
fi

echo "== Assert claim login has no direct access to queue or billing tables =="
claim_table_access="$(psql "$claim_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT has_table_privilege(session_user, 'public.bulk_import_jobs', 'SELECT')::text || ':' || has_table_privilege(session_user, 'public.billing_plans', 'SELECT')::text")"
if [[ "$claim_table_access" != "false:false" ]]; then
  echo "Queue-claim login unexpectedly has direct table access: $claim_table_access" >&2
  exit 1
fi
api_billing_access="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT has_table_privilege('stonez_api_runtime', 'public.billing_plans', 'SELECT')")"
if [[ "$api_billing_access" != "t" ]]; then
  echo "Fixture did not grant API runtime access needed to prove claim-login isolation" >&2
  exit 1
fi
api_claim_execute="$(psql "$runner_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT has_function_privilege('stonez_api_runtime', 'stonez_internal.claim_next_bulk_import_job(uuid)', 'EXECUTE')")"
if [[ "$api_claim_execute" != "f" ]]; then
  echo "API runtime unexpectedly received the queue-claim function grant" >&2
  exit 1
fi

echo "== Assert runtime can execute the installed SECURITY DEFINER claim function =="
claimed_rows="$(psql "$claim_url" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM stonez_internal.claim_next_bulk_import_job('44444444-4444-4444-8444-444444444444')")"
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
