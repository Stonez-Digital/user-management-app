-- Install the minimum public authentication lookup functions.
-- Run after provision_auth_bootstrap_roles.sql as the migration administrator.
SET ROLE stonez_auth_bootstrap_owner;

CREATE OR REPLACE FUNCTION stonez_internal.auth_login_lookup(p_email text)
RETURNS TABLE (
    user_id uuid,
    school_id uuid,
    full_name text,
    email text,
    password_hash text,
    role text,
    active boolean,
    school_status text,
    school_code text,
    school_slug text
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
    SELECT u.id, u.school_id, u.name, u.email, u.password_hash, u.role, u.active,
           s.status::text, s.code, s.slug
      FROM public.users AS u
      LEFT JOIN public.schools AS s ON s.id = u.school_id
     WHERE lower(u.email) = lower(btrim(p_email))
     LIMIT 1
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_refresh_lookup(p_token_hash text)
RETURNS TABLE (
    token_id uuid,
    family_id uuid,
    user_id uuid,
    token_expires_at timestamptz,
    token_revoked boolean,
    session_id uuid,
    session_expires_at timestamptz,
    session_revoked boolean,
    school_id uuid,
    role text,
    active boolean,
    school_status text
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
    SELECT rt.id, rt.family_id, rt.user_id, rt.expires_at, rt.revoked,
           COALESCE(ss.id, '00000000-0000-0000-0000-000000000000'::uuid),
           COALESCE(ss.expires_at, '0001-01-01 00:00:00+00'::timestamptz),
           COALESCE(ss.revoked, true),
           u.school_id, u.role, u.active, sc.status::text
      FROM public.refresh_tokens AS rt
      JOIN public.users AS u ON u.id = rt.user_id
      LEFT JOIN public.schools AS sc ON sc.id = u.school_id
      LEFT JOIN public.sessions AS ss
        ON ss.user_id = rt.user_id
       AND ss.refresh_token_hash = rt.token_hash
     WHERE rt.token_hash = p_token_hash
     LIMIT 1
$function$;

REVOKE ALL ON FUNCTION stonez_internal.auth_login_lookup(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_refresh_lookup(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_login_lookup(text) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_refresh_lookup(text) TO stonez_api_runtime;

RESET ROLE;


-- Keep token/session writes behind a second non-login owner. The API role
-- receives EXECUTE only; it never receives direct table DML or token-hash reads.
SET ROLE stonez_auth_session_owner;

CREATE OR REPLACE FUNCTION stonez_internal.auth_create_session(
    p_refresh_id uuid,
    p_user_id uuid,
    p_family_id uuid,
    p_refresh_hash text,
    p_expires_at timestamptz,
    p_session_id uuid,
    p_user_agent text,
    p_ip text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    INSERT INTO public.refresh_tokens (id, user_id, family_id, token_hash, expires_at, revoked)
    VALUES (p_refresh_id, p_user_id, p_family_id, p_refresh_hash, p_expires_at, false);
    INSERT INTO public.sessions (id, user_id, family_id, refresh_token_hash, expires_at, revoked, user_agent, ip)
    VALUES (p_session_id, p_user_id, p_family_id, p_refresh_hash, p_expires_at, false, p_user_agent, p_ip);
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_rotate_refresh(
    p_old_token_id uuid,
    p_old_hash text,
    p_new_refresh_id uuid,
    p_user_id uuid,
    p_family_id uuid,
    p_new_hash text,
    p_new_expires_at timestamptz,
    p_session_id uuid
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    -- Lock the session first. Competing rotations for this session serialize
    -- here; the second request observes the changed hash after the first commits.
    PERFORM 1
      FROM public.sessions
     WHERE id = p_session_id
       AND user_id = p_user_id
       AND family_id = p_family_id
       AND refresh_token_hash = p_old_hash
       AND revoked = false
       AND expires_at > clock_timestamp()
     FOR UPDATE;
    IF NOT FOUND THEN
        RETURN false;
    END IF;

    UPDATE public.refresh_tokens
       SET revoked = true
     WHERE id = p_old_token_id
       AND token_hash = p_old_hash
       AND user_id = p_user_id
       AND family_id = p_family_id
       AND revoked = false
       AND expires_at > clock_timestamp();
    IF NOT FOUND THEN
        RETURN false;
    END IF;

    INSERT INTO public.refresh_tokens (id, user_id, family_id, token_hash, expires_at, revoked)
    VALUES (p_new_refresh_id, p_user_id, p_family_id, p_new_hash, p_new_expires_at, false);

    UPDATE public.sessions
       SET refresh_token_hash = p_new_hash, expires_at = p_new_expires_at
     WHERE id = p_session_id
       AND user_id = p_user_id
       AND family_id = p_family_id
       AND refresh_token_hash = p_old_hash
       AND revoked = false;
    IF NOT FOUND THEN
        -- An unexpected post-lock mismatch must abort the entire SQL statement,
        -- including the preceding token revocation and replacement insert.
        RAISE EXCEPTION 'refresh session changed during locked rotation';
    END IF;
    RETURN true;
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_revoke_refresh_family(p_family_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    -- Keep the same lock order as auth_rotate_refresh (session, then token)
    -- to avoid deadlocks between family revocation and an in-flight rotation.
    UPDATE public.sessions SET revoked = true WHERE family_id = p_family_id;
    UPDATE public.refresh_tokens SET revoked = true WHERE family_id = p_family_id;
END
$function$;

REVOKE ALL ON FUNCTION stonez_internal.auth_create_session(uuid, uuid, uuid, text, timestamptz, uuid, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_rotate_refresh(uuid, text, uuid, uuid, uuid, text, timestamptz, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_revoke_refresh_family(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_create_session(uuid, uuid, uuid, text, timestamptz, uuid, text, text) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_rotate_refresh(uuid, text, uuid, uuid, uuid, text, timestamptz, uuid) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_revoke_refresh_family(uuid) TO stonez_api_runtime;

RESET ROLE;


-- Logout and device-session revocation remain behind the restricted writer.
-- All functions lock sessions before refresh tokens, matching refresh rotation.
SET ROLE stonez_auth_session_owner;

CREATE OR REPLACE FUNCTION stonez_internal.auth_logout(p_token_hash text)
RETURNS TABLE(user_id uuid, school_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE
    v_session_id uuid;
    v_user_id uuid;
    v_school_id uuid;
BEGIN
    SELECT s.id, s.user_id, u.school_id
      INTO v_session_id, v_user_id, v_school_id
      FROM public.sessions AS s
      JOIN public.users AS u ON u.id = s.user_id
     WHERE s.refresh_token_hash = p_token_hash
       AND s.revoked = false
     FOR UPDATE OF s;
    IF NOT FOUND THEN
        RETURN;
    END IF;

    UPDATE public.sessions
       SET revoked = true
     WHERE id = v_session_id
       AND user_id = v_user_id
       AND revoked = false;

    UPDATE public.refresh_tokens
       SET revoked = true
     WHERE token_hash = p_token_hash
       AND user_id = v_user_id
       AND revoked = false;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'logout refresh token missing or already revoked';
    END IF;

    RETURN QUERY SELECT v_user_id, v_school_id;
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_logout_all(p_user_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    UPDATE public.sessions SET revoked = true
     WHERE user_id = p_user_id AND revoked = false;
    UPDATE public.refresh_tokens SET revoked = true
     WHERE user_id = p_user_id AND revoked = false;
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_revoke_session(p_session_id uuid, p_user_id uuid)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE
    v_refresh_hash text;
BEGIN
    SELECT s.refresh_token_hash
      INTO v_refresh_hash
      FROM public.sessions AS s
     WHERE s.id = p_session_id
       AND s.user_id = p_user_id
       AND s.revoked = false
     FOR UPDATE;
    IF NOT FOUND THEN
        RETURN false;
    END IF;

    UPDATE public.sessions
       SET revoked = true
     WHERE id = p_session_id
       AND user_id = p_user_id
       AND revoked = false;

    UPDATE public.refresh_tokens
       SET revoked = true
     WHERE token_hash = v_refresh_hash
       AND user_id = p_user_id
       AND revoked = false;
    RETURN true;
END
$function$;

REVOKE ALL ON FUNCTION stonez_internal.auth_logout(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_logout_all(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_revoke_session(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_logout(text) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_logout_all(uuid) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_revoke_session(uuid, uuid) TO stonez_api_runtime;

RESET ROLE;
-- Password recovery lookups use the read-only bootstrap owner. Password and
-- reset-token writes use the restricted session writer owner.
SET ROLE stonez_auth_bootstrap_owner;

CREATE OR REPLACE FUNCTION stonez_internal.auth_password_reset_lookup(p_email text)
RETURNS TABLE(user_id uuid, full_name text, email text, school_id uuid)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
    SELECT u.id, u.name, u.email, u.school_id
      FROM public.users AS u
     WHERE lower(u.email) = lower(btrim(p_email))
       AND u.active = true
     LIMIT 1
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_password_change_lookup(p_user_id uuid)
RETURNS TABLE(user_id uuid, password_hash text, school_id uuid)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
    SELECT u.id, u.password_hash, u.school_id
      FROM public.users AS u
     WHERE u.id = p_user_id
       AND u.active = true
     LIMIT 1
$function$;

REVOKE ALL ON FUNCTION stonez_internal.auth_password_reset_lookup(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_password_change_lookup(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_password_reset_lookup(text) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_password_change_lookup(uuid) TO stonez_api_runtime;

RESET ROLE;
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_auth_bootstrap_owner;

GRANT USAGE, CREATE ON SCHEMA stonez_internal TO stonez_auth_session_owner;
SET ROLE stonez_auth_session_owner;

CREATE OR REPLACE FUNCTION stonez_internal.auth_create_password_reset(
    p_id uuid, p_user_id uuid, p_token_hash text, p_expires_at timestamptz
)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
    INSERT INTO public.password_reset_tokens (id, user_id, token_hash, expires_at, used, created_at)
    VALUES (p_id, p_user_id, p_token_hash, p_expires_at, false, clock_timestamp())
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_disable_password_reset(p_id uuid)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    UPDATE public.password_reset_tokens SET used = true WHERE id = p_id AND used = false;
    RETURN FOUND;
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_consume_password_reset(p_token_hash text, p_password_hash text)
RETURNS TABLE(user_id uuid, school_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE
    v_reset_id uuid;
    v_user_id uuid;
    v_school_id uuid;
BEGIN
    SELECT pr.id, pr.user_id, u.school_id
      INTO v_reset_id, v_user_id, v_school_id
      FROM public.password_reset_tokens AS pr
      JOIN public.users AS u ON u.id = pr.user_id
     WHERE pr.token_hash = p_token_hash
       AND pr.used = false
       AND pr.expires_at > clock_timestamp()
       AND u.active = true
     FOR UPDATE OF pr, u;
    IF NOT FOUND THEN
        RETURN;
    END IF;

    UPDATE public.password_reset_tokens
       SET used = true
     WHERE id = v_reset_id AND used = false;
    IF NOT FOUND THEN
        RETURN;
    END IF;

    UPDATE public.users
       SET password_hash = p_password_hash
     WHERE id = v_user_id AND active = true;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'password reset account became inactive';
    END IF;

    -- Keep lock order aligned with refresh rotation and logout.
    UPDATE public.sessions SET revoked = true
     WHERE user_id = v_user_id AND revoked = false;
    UPDATE public.refresh_tokens SET revoked = true
     WHERE user_id = v_user_id AND revoked = false;

    RETURN QUERY SELECT v_user_id, v_school_id;
END
$function$;

CREATE OR REPLACE FUNCTION stonez_internal.auth_revoke_user_sessions(p_user_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    UPDATE public.sessions SET revoked = true
     WHERE user_id = p_user_id AND revoked = false;
    UPDATE public.refresh_tokens SET revoked = true
     WHERE user_id = p_user_id AND revoked = false;
END
$function$;

REVOKE ALL ON FUNCTION stonez_internal.auth_revoke_user_sessions(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_revoke_user_sessions(uuid) TO stonez_auth_bootstrap_owner;

REVOKE ALL ON FUNCTION stonez_internal.auth_create_password_reset(uuid,uuid,text,timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_disable_password_reset(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION stonez_internal.auth_consume_password_reset(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_create_password_reset(uuid,uuid,text,timestamptz) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_disable_password_reset(uuid) TO stonez_api_runtime;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_consume_password_reset(text,text) TO stonez_api_runtime;

RESET ROLE;
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_auth_session_owner;

SET ROLE stonez_auth_bootstrap_owner;

CREATE OR REPLACE FUNCTION stonez_internal.auth_change_password(p_user_id uuid, p_expected_password_hash text, p_password_hash text)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    UPDATE public.users
       SET password_hash = p_password_hash
     WHERE id = p_user_id AND active = true AND password_hash = p_expected_password_hash;
    IF NOT FOUND THEN
        RETURN false;
    END IF;
    PERFORM stonez_internal.auth_revoke_user_sessions(p_user_id);
    RETURN true;
END
$function$;

REVOKE ALL ON FUNCTION stonez_internal.auth_change_password(uuid,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stonez_internal.auth_change_password(uuid,text,text) TO stonez_api_runtime;

RESET ROLE;
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_auth_bootstrap_owner;
