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
    SELECT rt.id, rt.family_id, rt.user_id, rt.expires_at, rt.revoked,
           COALESCE(ss.id, '00000000-0000-0000-0000-000000000000'::uuid),
           COALESCE(ss.expires_at, '-infinity'::timestamptz),
           COALESCE(ss.revoked, true),
           u.school_id, u.name, u.email, u.password_hash, u.role, u.active,
           sc.status::text, sc.code, sc.slug
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
REVOKE CREATE ON SCHEMA stonez_internal FROM stonez_auth_bootstrap_owner;
