# Authentication bootstrap database capability



## Scope

This is the first database-side capability slice for access-token authentication. It replaces neither HTTP middleware nor the shared application connection yet. The SQL contract and disposable PostgreSQL test establish the narrow database primitive before wiring a dedicated connection pool.

The capability validates an access-token session and returns only authenticated user ID, current database role, and current school ID. It does not return email, name, password hashes, refresh-token hashes, reset-token hashes, or arbitrary user columns. Login/password verification, refresh rotation, logout, password reset, and session-management writes remain separate capabilities.

## Role model

- stonez_auth_function_owner: NOLOGIN, NOSUPERUSER, NOBYPASSRLS, no database/role creation and no replication. It owns only the fixed-purpose function.

- stonez_auth_runtime: initially NOLOGIN, NOSUPERUSER, NOBYPASSRLS, no database/role creation and no replication. A later connection-pool change may provision login capability and credentials independently.

The runtime role receives schema USAGE and EXECUTE on one function. It receives no table or column privileges and is not a member of the function-owner role.

## Function and RLS

The function authenticate_access_session(session_id,user_id) checks the exact session/user pair, revocation, expiry, and current user active status. It returns no row for invalid sessions or inactive users. Role and school are read from the current user row on every invocation.

The function is SECURITY DEFINER, has a fixed search_path, and uses schema-qualified relation names. Its owner receives only column-level SELECT on the four session columns and four user columns required by the lookup. The function-owner role is explicitly allowed by narrow SELECT policies on users and sessions; both tables use ENABLE ROW LEVEL SECURITY. The disposable test additionally applies FORCE ROW LEVEL SECURITY and verifies the function still works, proving compatibility with a later reviewed FORCE RLS cutover. The function owner is not a table owner and does not have BYPASSRLS.

## Staging boundary

This PR deliberately does not wire the function into AuthMiddleware or RequireSchoolContext, configure AUTH_BOOTSTRAP_DATABASE_URL or add a second pool, grant a password or LOGIN to the runtime role, move login/refresh/logout/reset/session-management operations, add tenant policies, or query/modify production. Until a separately reviewed connection/bootstrap change, the current application continues using its existing database path. Do not treat this SQL contract as an active runtime security boundary.

## Verification

TestAuthBootstrapCapabilityPostgres requires TEST_DATABASE_URL pointing to local disposable PostgreSQL. It creates an isolated schema and uniquely named roles, executes the SQL capability there, and verifies valid/invalid sessions, current role/school changes, denial of underlying table and sensitive-column access, restricted owner column privileges, forced RLS, role attributes/membership, and search_path resistance.
