-- Provision the non-login owner for the public authentication lookup boundary.
-- Run with the database migration administrator, never with the API runtime role.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_api_runtime') THEN
        RAISE EXCEPTION 'stonez_api_runtime must be provisioned before auth bootstrap';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner') THEN
        CREATE ROLE stonez_auth_bootstrap_owner;
    END IF;
END
$$;

ALTER ROLE stonez_auth_bootstrap_owner
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;

REVOKE stonez_auth_bootstrap_owner FROM stonez_api_runtime;
REVOKE ALL ON SCHEMA stonez_internal FROM PUBLIC;
GRANT USAGE ON SCHEMA stonez_internal TO stonez_api_runtime;
GRANT USAGE, CREATE ON SCHEMA stonez_internal TO stonez_auth_bootstrap_owner;
GRANT USAGE ON SCHEMA public TO stonez_auth_bootstrap_owner;

-- The owner can read only the columns needed to authenticate. It cannot
-- mutate credentials, tokens, sessions, school records, or any other table.
GRANT SELECT (id, school_id, name, email, password_hash, role, active)
    ON TABLE public.users TO stonez_auth_bootstrap_owner;
GRANT SELECT (id, status, code, slug)
    ON TABLE public.schools TO stonez_auth_bootstrap_owner;
GRANT SELECT (id, family_id, user_id, token_hash, expires_at, revoked)
    ON TABLE public.refresh_tokens TO stonez_auth_bootstrap_owner;
GRANT SELECT (id, user_id, family_id, refresh_token_hash, expires_at, revoked)
    ON TABLE public.sessions TO stonez_auth_bootstrap_owner;

-- These policies are dormant until RLS is enabled on the relevant tables.
-- They grant this non-login role SELECT only; they do not enable RLS or
-- change access for the existing API/worker roles.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'users' AND policyname = 'auth_bootstrap_owner_select') THEN
        CREATE POLICY auth_bootstrap_owner_select ON public.users FOR SELECT TO stonez_auth_bootstrap_owner USING (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'schools' AND policyname = 'auth_bootstrap_owner_select') THEN
        CREATE POLICY auth_bootstrap_owner_select ON public.schools FOR SELECT TO stonez_auth_bootstrap_owner USING (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'refresh_tokens' AND policyname = 'auth_bootstrap_owner_select') THEN
        CREATE POLICY auth_bootstrap_owner_select ON public.refresh_tokens FOR SELECT TO stonez_auth_bootstrap_owner USING (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'sessions' AND policyname = 'auth_bootstrap_owner_select') THEN
        CREATE POLICY auth_bootstrap_owner_select ON public.sessions FOR SELECT TO stonez_auth_bootstrap_owner USING (true);
    END IF;
END
$$;

-- Keep schema creation rights only for the provisioning step.
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_auth_bootstrap_owner;
