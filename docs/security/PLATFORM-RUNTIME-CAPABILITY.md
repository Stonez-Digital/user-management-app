# Platform runtime database capability

## Status

This is a staging-only capability boundary. It does not authorize production role creation, grants, policy installation, credential changes, or deployment. The platform runtime remains disabled until a reviewed table-access specification and real PostgreSQL 17 RLS tests are complete.

## Connection contract

- DATABASE_URL remains the legacy application connection during the transition.
- PLATFORM_DATABASE_URL is a separate PostgreSQL LOGIN for platform-only routes.
- The expected login is stonez_platform_runtime.
- REQUIRE_PLATFORM_DATABASE=true makes the dedicated connection mandatory and fails startup if it is missing, cannot be reached, or fails role validation.
- When the flag is false and no dedicated URL is configured, the server explicitly logs that legacy shared-connection mode is active. This compatibility mode is not evidence of isolation.
- Platform controllers and the national-curriculum service use the platform handle when it is configured.
- The platform pool is closed during shutdown.

## Startup role checks

The connection is rejected unless current_user and session_user both equal stonez_platform_runtime. The login must have LOGIN and must not be superuser, BYPASSRLS, CREATEDB, CREATEROLE, REPLICATION, or INHERIT. It must have no role memberships, own no public-schema relations, and have no CREATE privilege in the public schema.

These checks establish a distinct, non-owner, non-bypass identity. They do not establish least-privilege table grants or correct row policies. Those remain separate gates.

## Required next gates

1. Approve the platform-owned and cross-school read/write table inventory, including platform audit records and national curricula.
2. Specify direct grants and RLS policy behavior for this exact role; do not infer grants from the controller code alone.
3. Apply the actual versioned application migrations and reviewed policy SQL in disposable PostgreSQL 17.
4. Prove platform operations can perform only approved cross-school actions while tenant roles cannot invoke platform capabilities.
5. Verify audit writes, rollback, and failure behavior under the actual role.
6. Only after staging evidence and explicit approval should a separate production credential/role cutover be considered.

No migration or role-provisioning SQL is included here. The connection check is read-only.
