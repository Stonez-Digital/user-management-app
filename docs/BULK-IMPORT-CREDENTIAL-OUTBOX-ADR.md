# Bulk-import credential notification outbox ADR

- Status: Proposed
- Scope: PR #363, signed bulk-import worker
- Decision: Replace post-commit temporary-password email with a durable account-setup notification outbox.

## Context

The current worker creates a user and profile inside a tenant transaction, commits that transaction, and only then calls `SendCredentials`. The email contains a generated temporary password. If the process stops after commit and before successful delivery, a retry finds the existing account, skips creation, and no longer has the plaintext password. Reconstructing or persisting that password would expand credential exposure.

The existing password-reset flow already stores only a hash of a one-time token and delivers the token separately. The bulk-import outbox must not store temporary passwords, password hashes, reset tokens, signed worker assertions, or other bearer credentials.

## Decision

When `send_credentials` is enabled and a bulk import creates a new user, insert an account-setup notification record in the same database transaction as the user and profile rows. The record contains only the minimum delivery data: event ID, school ID, import job ID, user ID, recipient address, display name, role, state, attempt count, next-attempt time, lease metadata, timestamps, bounded last-error metadata, and an optional provider message identifier.

The email is an account-created notice with the portal login address and instructions to use the existing password-reset flow to choose a password. It must not contain or generate a temporary password or reset token. A reset token is created only by the existing password-reset request flow and remains outside the outbox.

## Database trust boundary

- Store the outbox in a private schema, not in a tenant-readable application table.
- Do not grant the processing login direct table SELECT, INSERT, UPDATE, or DELETE privileges.
- Expose only narrowly scoped `SECURITY DEFINER` functions with fixed `search_path` and explicit execution grants.
- Every enqueue, claim, completion, and retry operation must require a valid transaction-bound signed context and must bind the event to the current verified school and job.
- The function owner must be the dedicated context-owner role; it must not be the API runtime, queue owner, processing login, or key-admin role.
- Claim operations must lock and lease only pending/due events for the current verified job and school. Completion/retry must require the exact event ID and lease generation returned by claim.
- Do not add a broad RLS policy or table grant to make the outbox accessible.

## Delivery and recovery semantics

Delivery is at-least-once. A worker can crash after the provider accepts an email but before the outbox row is marked delivered, so a duplicate account-setup notice is possible. The notice must be safe to receive more than once. Use a provider idempotency key where the provider supports it, derived from the stable event ID; do not claim exactly-once delivery.

The worker performs network delivery outside the database transaction. It claims an event in a short transaction, sends the notice, then records delivered or schedules a bounded retry in a separate signed-context transaction. A lease expiry makes abandoned claims eligible for retry after worker restart. Retry delay must be bounded and use backoff; permanent recipient/configuration failures must be visible for operator action rather than silently discarded.

The account/profile/outbox insert is atomic. If any outbox insert fails, the account/profile transaction rolls back. A retry after a crash sees the durable pending event and can deliver it even though account creation is now a no-op.

## Required tests before enabling the feature

1. Account/profile creation and outbox insertion commit together; injected outbox failure rolls all of them back.
2. Crash after account/outbox commit but before delivery; a new worker claims and sends the same event.
3. Crash after provider acceptance but before marking delivered; retry is safe and duplicate delivery is documented.
4. Two workers cannot hold the same active event lease concurrently.
5. Expired leases can be reclaimed; stale lease holders cannot complete or retry a reclaimed event.
6. Cross-school and cross-job reads, claims, updates, and deletes fail.
7. Missing, expired, forged, replayed, or wrong-lease signed context cannot access the outbox.
8. Processing login has no direct table privileges; unrelated logins cannot execute outbox functions.
9. No temporary password, password hash, reset token, or signed assertion appears in outbox rows, errors, logs, or email content.
10. Provider failure increments attempts, records bounded diagnostic metadata, and schedules retry without losing the event.
11. A worker restart resumes pending and expired-lease events without creating duplicate accounts.
12. Existing credential-email counters and job status accurately distinguish queued, delivered, and permanently failed notices.

## Rollout constraints

This ADR is design-only until the SQL boundary, Go worker integration, isolated PostgreSQL tests, and operator recovery documentation are reviewed. The installer remains operator-run. No staging or production installation, secret change, deployment, or merge is authorized by this decision. PR #363 remains draft and unmerged.
