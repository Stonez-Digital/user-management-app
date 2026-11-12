-- Reconstructed source for the already-applied backend-only access migration.
-- Verified against project tthzvaguegpmcbxqfcms on 2026-11-12:
-- career_path_attempts has RLS enabled and table privileges only for
-- postgres and service_role (no anon/authenticated grants).
-- The table itself is provisioned by the application schema migration path.

ALTER TABLE public.career_path_attempts ENABLE ROW LEVEL SECURITY;

REVOKE ALL PRIVILEGES ON TABLE public.career_path_attempts FROM anon, authenticated;
GRANT ALL PRIVILEGES ON TABLE public.career_path_attempts TO service_role;
