#!/usr/bin/env bash
set -euo pipefail

: "${PGHOST:=localhost}"
: "${PGPORT:=5432}"
: "${PGUSER:=postgres}"
: "${PGPASSWORD:=postgres}"
export PGHOST PGPORT PGUSER PGPASSWORD

suffix="${BASHPID:-$$}"
admin_db="postgres"
psql_admin() {
  psql -X -v ON_ERROR_STOP=1 -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$admin_db" "$@"
}
psql_db() {
  local db="$1"
  shift
  psql -X -v ON_ERROR_STOP=1 -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$db" "$@"
}

for extension_schema in public extensions; do
  db="bulk_context_pgcrypto_${extension_schema}_${suffix}"
  owner="bctx_owner_${suffix}"
  process="bctx_process_${suffix}"

  cleanup() {
    psql_admin -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$db' AND pid <> pg_backend_pid()" >/dev/null
    psql_admin -c "DROP DATABASE IF EXISTS \"$db\"" >/dev/null
    psql_admin -c "DROP ROLE IF EXISTS $owner; DROP ROLE IF EXISTS $process" >/dev/null
  }
  trap cleanup EXIT

  psql_admin -c "CREATE ROLE $owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS"
  psql_admin -c "CREATE ROLE $process NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS"
  psql_admin -c "CREATE DATABASE \"$db\""

  if [[ "$extension_schema" == "extensions" ]]; then
    psql_db "$db" -c "CREATE SCHEMA extensions; CREATE EXTENSION pgcrypto WITH SCHEMA extensions"
  else
    psql_db "$db" -c "CREATE EXTENSION pgcrypto WITH SCHEMA public"
  fi

  psql_db "$db" <<'SQL'
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
    lease_generation bigint NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    rows_json text NOT NULL DEFAULT '[]',
    errors_json text NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
SQL

  sed \
    -e "s/stonez_bulk_context_owner/$owner/g" \
    -e "s/stonez_bulk_process_group/$process/g" \
    scripts/security/install_bulk_import_signed_context.sql | psql_db "$db"

  sed \
    -e "s/stonez_bulk_context_owner/$owner/g" \
    -e "s/stonez_bulk_process_group/$process/g" \
    scripts/security/verify_bulk_import_signed_context.sql | psql_db "$db"

  echo "PASS: signed-context installer and verifier with pgcrypto in $extension_schema"
  cleanup
  trap - EXIT
done
