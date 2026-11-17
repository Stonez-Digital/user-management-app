-- Operator/maintenance-role cleanup for expired transaction context replay records.
-- Run in bounded batches; this never deletes unexpired assertions.
WITH expired AS (
    SELECT ctid
      FROM stonez_internal.bulk_context_sessions
     WHERE expires_at < clock_timestamp() - interval '5 minutes'
     ORDER BY expires_at
     LIMIT 1000
)
DELETE FROM stonez_internal.bulk_context_sessions AS session_row
 USING expired
 WHERE session_row.ctid = expired.ctid;
