-- Fail closed if the authentication bootstrap boundary is incomplete.
DO $$
DECLARE
    v_role pg_roles%ROWTYPE;
    v_owner oid;
    v_security_definer boolean;
    v_config text[];
BEGIN
    SELECT * INTO v_role FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner';
    IF NOT FOUND OR v_role.rolcanlogin OR v_role.rolsuper OR v_role.rolcreatedb
       OR v_role.rolcreaterole OR v_role.rolbypassrls OR v_role.rolinherit THEN
        RAISE EXCEPTION 'auth bootstrap owner role is missing or has unsafe attributes';
    END IF;
    IF pg_has_role('stonez_api_runtime', 'stonez_auth_bootstrap_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'API runtime must not be a member of the auth bootstrap owner role';
    END IF;

    SELECT * INTO v_role FROM pg_roles WHERE rolname = 'stonez_auth_session_owner';
    IF NOT FOUND OR v_role.rolcanlogin OR v_role.rolsuper OR v_role.rolcreatedb
       OR v_role.rolcreaterole OR v_role.rolbypassrls OR v_role.rolinherit THEN
        RAISE EXCEPTION 'auth session owner role is missing or has unsafe attributes';
    END IF;
    IF pg_has_role('stonez_api_runtime', 'stonez_auth_session_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'API runtime must not be a member of the auth session owner role';
    END IF;

    SELECT p.proowner, p.prosecdef, p.proconfig
      INTO v_owner, v_security_definer, v_config
      FROM pg_proc AS p
      JOIN pg_namespace AS n ON n.oid = p.pronamespace
     WHERE n.nspname = 'stonez_internal'
       AND p.proname = 'auth_login_lookup'
       AND p.pronargs = 1 AND p.proargtypes[0] = 'text'::regtype;
    IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner')
       OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
        RAISE EXCEPTION 'auth_login_lookup owner/SECURITY DEFINER/search_path verification failed';
    END IF;

    SELECT p.proowner, p.prosecdef, p.proconfig
      INTO v_owner, v_security_definer, v_config
      FROM pg_proc AS p
      JOIN pg_namespace AS n ON n.oid = p.pronamespace
     WHERE n.nspname = 'stonez_internal'
       AND p.proname = 'auth_refresh_lookup'
       AND p.pronargs = 1 AND p.proargtypes[0] = 'text'::regtype;
    IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner')
       OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
        RAISE EXCEPTION 'auth_refresh_lookup owner/SECURITY DEFINER/search_path verification failed';
    END IF;

    IF NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_login_lookup(text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_refresh_lookup(text)', 'EXECUTE') THEN
        RAISE EXCEPTION 'API runtime is missing required auth bootstrap EXECUTE grants';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
          CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) AS a
         WHERE n.nspname = 'stonez_internal'
           AND p.proname IN ('auth_login_lookup', 'auth_refresh_lookup')
           AND a.grantee = 0
           AND a.privilege_type = 'EXECUTE'
    ) THEN
        RAISE EXCEPTION 'PUBLIC must not execute auth bootstrap functions';
    END IF;

    IF NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_create_session(uuid,uuid,uuid,text,timestamptz,uuid,text,text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_rotate_refresh(uuid,text,uuid,uuid,uuid,text,timestamptz,uuid)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_revoke_refresh_family(uuid)', 'EXECUTE') THEN
        RAISE EXCEPTION 'API runtime is missing required auth session write EXECUTE grants';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
          CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) AS a
         WHERE n.nspname = 'stonez_internal'
           AND p.proname IN ('auth_create_session', 'auth_rotate_refresh', 'auth_revoke_refresh_family')
           AND (a.grantee = 0 AND a.privilege_type = 'EXECUTE')
    ) THEN
        RAISE EXCEPTION 'PUBLIC must not execute auth session write functions';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
         WHERE n.nspname = 'stonez_internal'
           AND p.proname IN ('auth_create_session', 'auth_rotate_refresh', 'auth_revoke_refresh_family')
           AND (p.proowner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_session_owner')
                OR NOT p.prosecdef OR NOT ('search_path=pg_catalog' = ANY(p.proconfig))
    )) THEN
        RAISE EXCEPTION 'auth session write function owner/SECURITY DEFINER/search_path verification failed';
    END IF;
END
$$;

-- Verify logout/session-revocation functions have the same restricted owner,
-- fixed search path, and API-only EXECUTE grants.
DO $logout_boundary$
DECLARE
    v_name text;
    v_owner oid;
    v_security_definer boolean;
    v_config text[];
BEGIN
    FOREACH v_name IN ARRAY ARRAY['auth_logout', 'auth_logout_all', 'auth_revoke_session'] LOOP
        SELECT p.proowner, p.prosecdef, p.proconfig
          INTO v_owner, v_security_definer, v_config
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
         WHERE n.nspname = 'stonez_internal'
           AND p.proname = v_name;
        IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_session_owner')
           OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
            RAISE EXCEPTION '% owner/SECURITY DEFINER/search_path verification failed', v_name;
        END IF;
        IF EXISTS (
            SELECT 1
              FROM pg_proc AS p
              JOIN pg_namespace AS n ON n.oid = p.pronamespace
              CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) AS a
             WHERE n.nspname = 'stonez_internal'
               AND p.proname = v_name
               AND a.grantee = 0
               AND a.privilege_type = 'EXECUTE'
        ) THEN
            RAISE EXCEPTION 'PUBLIC must not execute %', v_name;
        END IF;
    END LOOP;
    IF NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_logout(text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_logout_all(uuid)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_revoke_session(uuid,uuid)', 'EXECUTE') THEN
        RAISE EXCEPTION 'API runtime is missing logout/session revocation EXECUTE grants';
    END IF;
END
$logout_boundary$;

-- Verify password lookup functions stay with the read-only bootstrap owner
-- and credential/reset writers stay with the restricted session writer owner.
DO $password_boundary$
DECLARE
    v_name text;
    v_owner oid;
    v_security_definer boolean;
    v_config text[];
BEGIN
    FOREACH v_name IN ARRAY ARRAY['auth_password_reset_lookup', 'auth_password_change_lookup'] LOOP
        SELECT p.proowner, p.prosecdef, p.proconfig
          INTO v_owner, v_security_definer, v_config
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
         WHERE n.nspname = 'stonez_internal' AND p.proname = v_name;
        IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner')
           OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
            RAISE EXCEPTION '% owner/SECURITY DEFINER/search_path verification failed', v_name;
        END IF;
    END LOOP;

    FOREACH v_name IN ARRAY ARRAY['auth_create_password_reset', 'auth_disable_password_reset', 'auth_consume_password_reset', 'auth_revoke_user_sessions'] LOOP
        SELECT p.proowner, p.prosecdef, p.proconfig
          INTO v_owner, v_security_definer, v_config
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
         WHERE n.nspname = 'stonez_internal' AND p.proname = v_name;
        IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_session_owner')
           OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
            RAISE EXCEPTION '% owner/SECURITY DEFINER/search_path verification failed', v_name;
        END IF;
        IF EXISTS (
            SELECT 1
              FROM pg_proc AS p
              JOIN pg_namespace AS n ON n.oid = p.pronamespace
              CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) AS a
             WHERE n.nspname = 'stonez_internal' AND p.proname = v_name
               AND a.grantee = 0 AND a.privilege_type = 'EXECUTE'
        ) THEN
            RAISE EXCEPTION 'PUBLIC must not execute %', v_name;
        END IF;
    END LOOP;

    SELECT p.proowner, p.prosecdef, p.proconfig
      INTO v_owner, v_security_definer, v_config
      FROM pg_proc AS p
      JOIN pg_namespace AS n ON n.oid = p.pronamespace
     WHERE n.nspname = 'stonez_internal' AND p.proname = 'auth_change_password'
       AND p.pronargs = 3;
    IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner')
       OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
        RAISE EXCEPTION 'auth_change_password owner/SECURITY DEFINER/search_path verification failed';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid = p.pronamespace
          CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) AS a
         WHERE n.nspname = 'stonez_internal' AND p.proname = 'auth_change_password'
           AND a.grantee = 0 AND a.privilege_type = 'EXECUTE'
    ) THEN
        RAISE EXCEPTION 'PUBLIC must not execute auth_change_password';
    END IF;

    IF NOT has_function_privilege('stonez_auth_bootstrap_owner', 'stonez_internal.auth_revoke_user_sessions(uuid)', 'EXECUTE')
       OR has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_revoke_user_sessions(uuid)', 'EXECUTE') THEN
        RAISE EXCEPTION 'password change session-revocation helper grants are unsafe';
    END IF;

    IF NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_password_reset_lookup(text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_password_change_lookup(uuid)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_create_password_reset(uuid,uuid,text,timestamptz)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_disable_password_reset(uuid)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_consume_password_reset(text,text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_change_password(uuid,text,text)', 'EXECUTE') THEN
        RAISE EXCEPTION 'API runtime is missing password recovery/reset EXECUTE grants';
    END IF;
END
$password_boundary$;
