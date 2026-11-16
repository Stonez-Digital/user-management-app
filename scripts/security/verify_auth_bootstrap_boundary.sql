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

    SELECT p.proowner, p.prosecdef, p.proconfig
      INTO v_owner, v_security_definer, v_config
      FROM pg_proc AS p
      JOIN pg_namespace AS n ON n.oid = p.pronamespace
     WHERE n.nspname = 'stonez_internal'
       AND p.proname = 'auth_login_lookup'
       AND pg_get_function_identity_arguments(p.oid) = 'p_email text';
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
       AND pg_get_function_identity_arguments(p.oid) = 'p_token_hash text';
    IF NOT FOUND OR v_owner <> (SELECT oid FROM pg_roles WHERE rolname = 'stonez_auth_bootstrap_owner')
       OR NOT v_security_definer OR NOT ('search_path=pg_catalog' = ANY(v_config)) THEN
        RAISE EXCEPTION 'auth_refresh_lookup owner/SECURITY DEFINER/search_path verification failed';
    END IF;

    IF NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_login_lookup(text)', 'EXECUTE')
       OR NOT has_function_privilege('stonez_api_runtime', 'stonez_internal.auth_refresh_lookup(text)', 'EXECUTE') THEN
        RAISE EXCEPTION 'API runtime is missing required auth bootstrap EXECUTE grants';
    END IF;
    IF has_function_privilege('public', 'stonez_internal.auth_login_lookup(text)', 'EXECUTE')
       OR has_function_privilege('public', 'stonez_internal.auth_refresh_lookup(text)', 'EXECUTE') THEN
        RAISE EXCEPTION 'PUBLIC must not execute auth bootstrap functions';
    END IF;
END
$$;
