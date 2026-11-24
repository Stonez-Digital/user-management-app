# School-content media cleanup outbox

## Status

Design contract only. The durable outbox, worker, migration, and storage integration are **not implemented yet**. Do not treat current best-effort object deletion as reliable recovery, and do not merge or deploy this PR on the strength of this document.

## Why an outbox is required

PostgreSQL transactions cannot include Supabase Storage operations. A handler can upload an object and then have its SQL transaction roll back, or commit a row change while object deletion fails. Deleting an object synchronously before the SQL transaction commits is also unsafe: the SQL rollback could leave a live row pointing to a deleted object.

## Required transaction contract

1. Upload a replacement object first, but do not delete the previously referenced object.
2. In the same tenant transaction, update the row and enqueue cleanup for the previous object. Commit makes the new reference and cleanup job visible atomically.
3. For a database/audit failure after a new upload, enqueue compensation for the newly uploaded object through a separate recovery-capable database handle after the tenant transaction has rolled back. If the recovery write also fails, emit a high-severity structured error containing the school ID and object key, never credentials.
4. For row deletion, enqueue cleanup for every owned object in the same tenant transaction before deleting the rows. Cover blog featured images, gallery image objects, album cover images, advertisement images, and replaced school logos.
5. Never delete an old object before the SQL transaction commits.

## Outbox record contract

The versioned migration should create a school-owned cleanup queue with:

- UUID job ID and non-null school ID foreign key.
- A strict bucket enum/allowlist (school-media or school-logos).
- Object key, not an arbitrary caller-supplied URL.
- State (pending, processing, completed), attempt count, next-attempt time, lease timestamp/worker ID, last error, completion time, and timestamps.
- Indexes for due pending jobs and stale leases.
- RLS enabled and no grants to public Data API roles.
- No BYPASSRLS, superuser, table-owner, or general platform privileges for the worker.

Object keys must begin with the exact school_id/ prefix. URL parsing must require the configured Supabase origin, expected public-object route, and exact bucket before extracting the key. Unknown/legacy external URLs must not be queued for deletion.

## Worker contract

- Use a dedicated worker database capability/credential, not the tenant runtime or migration credential.
- Claim a bounded batch atomically with a lease (PostgreSQL FOR UPDATE SKIP LOCKED or a reviewed narrow claim function).
- Perform Storage deletion outside the SQL claim transaction.
- Make deletion idempotent; only a provider-confirmed missing-object response may count as already complete.
- Mark successful jobs completed. On failure, retain the job, record a bounded/sanitized error, and schedule exponential backoff with a maximum delay.
- Reclaim expired leases after worker crashes. Limit attempts only with a visible terminal/dead-letter state and an operator recovery path; never silently discard jobs.
- Do not start the worker until its restricted role, grants/policies, secret access, and staging tests are verified.

## Required verification before merge

- PostgreSQL 17 tests apply the actual versioned migration and exercise claim/lease/retry behavior.
- School A cannot enqueue, inspect, claim, or alter School B tenant jobs through tenant routes.
- Worker tests prove it can process only the queue capability and cannot read or mutate ordinary school records.
- Tests cover upload success + SQL failure compensation, replace, delete, storage timeout, duplicate delivery, worker crash/lease expiry, and retries.
- Audit writes and the content mutation share the same tenant transaction; audit failure rolls back the content row and any transactional cleanup job.
- Route tests cover all media-bearing mutation handlers.
- Staging migration parity and rollback evidence exist. Production remains untouched until separate authorization and all rollout gates pass.
