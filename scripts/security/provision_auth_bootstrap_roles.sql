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
DO $session_owner$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stonez_auth_session_owner') THEN
        CREATE ROLE stonez_auth_session_owner;
    END IF;
END
$session_owner$;

ALTER ROLE stonez_auth_session_owner
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;

REVOKE stonez_auth_session_owner FROM stonez_api_runtime;
GRANT stonez_auth_bootstrap_owner, stonez_auth_session_owner TO CURRENT_USER;
GRANT USAGE, CREATE ON SCHEMA stonez_internal TO stonez_auth_session_owner;
GRANT USAGE ON SCHEMA public TO stonez_auth_session_owner;

-- This non-login owner is used only by SECURITY DEFINER session-write
-- functions. Column grants limit the data it can inspect or modify.
GRANT SELECT (id, family_id, user_id, token_hash, expires_at, revoked), INSERT, UPDATE (revoked)
    ON TABLE public.refresh_tokens TO stonez_auth_session_owner;
GRANT SELECT (id, user_id, family_id, refresh_token_hash, expires_at, revoked), INSERT,
    UPDATE (refresh_token_hash, expires_at, revoked)
    ON TABLE public.sessions TO stonez_auth_session_owner;

GRANT SELECT (id, school_id, active) ON TABLE public.users TO stonez_auth_session_owner;
GRANT UPDATE (password_hash) ON TABLE public.users TO stonez_auth_session_owner;
GRANT SELECT (id, user_id, token_hash, expires_at, used), INSERT, UPDATE (used)
    ON TABLE public.password_reset_tokens TO stonez_auth_session_owner;

DO $session_policy$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'refresh_tokens' AND policyname = 'auth_session_owner_access') THEN
        CREATE POLICY auth_session_owner_access ON public.refresh_tokens FOR ALL TO stonez_auth_session_owner USING (true) WITH CHECK (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'sessions' AND policyname = 'auth_session_owner_access') THEN
        CREATE POLICY auth_session_owner_access ON public.sessions FOR ALL TO stonez_auth_session_owner USING (true) WITH CHECK (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'users' AND policyname = 'auth_session_owner_password_select') THEN
        CREATE POLICY auth_session_owner_password_select ON public.users FOR SELECT TO stonez_auth_session_owner USING (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'users' AND policyname = 'auth_session_owner_password_update') THEN
        CREATE POLICY auth_session_owner_password_update ON public.users FOR UPDATE TO stonez_auth_session_owner USING (true) WITH CHECK (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public' AND tablename = 'password_reset_tokens' AND policyname = 'auth_session_owner_reset_access') THEN
        CREATE POLICY auth_session_owner_reset_access ON public.password_reset_tokens FOR ALL TO stonez_auth_session_owner USING (true) WITH CHECK (true);
    END IF;
END
$session_policy$;
REVOKE ALL ON SCHEMA stonez_internal FROM PUBLIC;
GRANT USAGE ON SCHEMA stonez_internal TO stonez_api_runtime;
GRANT USAGE, CREATE ON SCHEMA stonez_internal TO stonez_auth_bootstrap_owner;
GRANT USAGE ON SCHEMA public TO stonez_auth_bootstrap_owner;

-- The owner can read only the columns needed to authenticate. It cannot
-- mutate credentials, tokens, sessions, school records, or any other table.
GRANT SELECT (id, school_id, name, email, password_hash, role, active)
    ON TABLE public.users TO stonez_auth_bootstrap_owner;
GRANT UPDATE (password_hash) ON TABLE public.users TO stonez_auth_bootstrap_owner;
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
