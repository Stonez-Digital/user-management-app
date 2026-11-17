-- Operator-run preparation for the signed-context verifier.
-- Creates only NOLOGIN group/owner roles. Does not provision keys or logins.
\set ON_ERROR_STOP on
BEGIN;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_context_owner') THEN
        CREATE ROLE stonez_bulk_context_owner
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_bulk_process_group') THEN
        CREATE ROLE stonez_bulk_process_group
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_roles
         WHERE rolname = 'stonez_bulk_context_owner'
           AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'stonez_bulk_context_owner has unsafe role attributes';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles
         WHERE rolname = 'stonez_bulk_process_group'
           AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR NOT rolinherit)
    ) THEN
        RAISE EXCEPTION 'stonez_bulk_process_group has unsafe role attributes';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles AS privileged
         WHERE (privileged.rolsuper OR privileged.rolbypassrls)
           AND pg_has_role('stonez_bulk_process_group', privileged.oid, 'MEMBER')
    ) THEN
        RAISE EXCEPTION 'stonez_bulk_process_group inherits a privileged role';
    END IF;
    IF pg_has_role('stonez_bulk_context_owner', 'stonez_bulk_process_group', 'MEMBER')
       OR pg_has_role('stonez_bulk_process_group', 'stonez_bulk_context_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'context owner and processing group must remain separate';
    END IF;
END
$$;
COMMIT;
