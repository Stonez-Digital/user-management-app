GRANT USAGE ON SCHEMA public TO stonez_auth_function_owner;
GRANT CREATE ON SCHEMA public TO stonez_auth_function_owner;
GRANT SELECT (id,user_id,expires_at,revoked) ON TABLE public.sessions TO stonez_auth_function_owner;
GRANT SELECT (id,active,role,school_id) ON TABLE public.users TO stonez_auth_function_owner;
ALTER TABLE public.sessions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS auth_bootstrap_function_owner_read ON public.sessions;
CREATE POLICY auth_bootstrap_function_owner_read ON public.sessions FOR SELECT TO stonez_auth_function_owner USING (true);
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS auth_bootstrap_function_owner_read ON public.users;
CREATE POLICY auth_bootstrap_function_owner_read ON public.users FOR SELECT TO stonez_auth_function_owner USING (true);
CREATE OR REPLACE FUNCTION public.authenticate_access_session(p_session_id uuid,p_user_id uuid)
RETURNS TABLE(user_id uuid,user_role text,school_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public
AS $function$ SELECT u.id,u.role,u.school_id FROM public.sessions AS s JOIN public.users AS u ON u.id=s.user_id WHERE s.id=p_session_id AND s.user_id=p_user_id AND s.revoked=false AND s.expires_at>statement_timestamp() AND u.active=true LIMIT 1 $function$;
ALTER FUNCTION public.authenticate_access_session(uuid,uuid) OWNER TO stonez_auth_function_owner;
REVOKE CREATE ON SCHEMA public FROM stonez_auth_function_owner;
REVOKE ALL ON FUNCTION public.authenticate_access_session(uuid,uuid) FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO stonez_auth_runtime;
GRANT EXECUTE ON FUNCTION public.authenticate_access_session(uuid,uuid) TO stonez_auth_runtime;
