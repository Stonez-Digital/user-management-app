-- Run from an operator connection whose session user is a member of
-- stonez_bulk_context_key_admin. Schedule periodically (for example hourly).
BEGIN;
SET LOCAL ROLE stonez_bulk_context_key_admin;
SELECT stonez_context_private.prune_expired_contexts() AS deleted_expired_contexts;
COMMIT;
