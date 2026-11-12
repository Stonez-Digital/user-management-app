-- Restrict privileges inherited by future objects created in public
-- by the postgres role.
--
-- Existing table/function/sequence grants are intentionally unchanged.
-- Client-facing access must be granted explicitly by a reviewed migration,
-- with RLS enabled before any table is exposed through the Supabase API.
-- service_role defaults are intentionally preserved.
--
-- This migration is executed as postgres. In this project, postgres is not
-- a member of supabase_admin, so PostgreSQL does not permit it to alter
-- supabase_admin's default privileges. Those defaults require a separate
-- platform/privileged-admin change and are deliberately not attempted here.
-- Do not add ALTER DEFAULT PRIVILEGES FOR ROLE supabase_admin here unless
-- the migration execution role is explicitly granted the required membership.

ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
  REVOKE ALL ON TABLES FROM anon, authenticated;

ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
  REVOKE ALL ON SEQUENCES FROM anon, authenticated;

ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
  REVOKE EXECUTE ON FUNCTIONS FROM anon, authenticated;
