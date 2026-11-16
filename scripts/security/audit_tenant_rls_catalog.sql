\set ON_ERROR_STOP on
\pset pager off
\echo 'Stonez Digital tenant/RLS catalog audit (read-only)'
\echo 'Run with: psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/security/audit_tenant_rls_catalog.sql'
\echo 'No DDL, DML, role changes, grants, or policy changes are performed.'

BEGIN TRANSACTION READ ONLY;

\echo ''
\echo '=== 1. Database/session identity ==='
SELECT current_database() AS database_name,
       current_user AS current_role_name,
       session_user AS session_user,
       current_setting('server_version') AS server_version,
       current_setting('transaction_read_only') AS transaction_read_only,
       r.rolsuper AS current_role_superuser,
       r.rolbypassrls AS current_role_bypassrls,
       r.rolinherit AS current_role_inherit
FROM pg_roles AS r
WHERE r.rolname = current_user;

\echo ''
\echo '=== 2. Public base/partitioned table inventory and RLS flags ==='
SELECT n.nspname AS schema_name,
       c.relname AS table_name,
       c.relkind AS relation_kind,
       pg_get_userbyid(c.relowner) AS owner,
       owner_role.rolsuper AS owner_superuser,
       owner_role.rolbypassrls AS owner_bypassrls,
       c.relrowsecurity AS rls_enabled,
       c.relforcerowsecurity AS rls_forced,
       c.relpersistence AS persistence,
       c.relispartition AS is_partition,
       COALESCE(policy_counts.policy_count, 0) AS policy_count
FROM pg_class AS c
JOIN pg_namespace AS n ON n.oid = c.relnamespace
JOIN pg_roles AS owner_role ON owner_role.oid = c.relowner
LEFT JOIN (
    SELECT schemaname, tablename, count(*) AS policy_count
    FROM pg_policies
    GROUP BY schemaname, tablename
) AS policy_counts
  ON policy_counts.schemaname = n.nspname
 AND policy_counts.tablename = c.relname
WHERE n.nspname = 'public'
  AND c.relkind IN ('r', 'p')
ORDER BY c.relname;

\echo ''
\echo '=== 3. Inventory count (45 is the previously documented expected count) ==='
SELECT count(*) AS public_base_and_partitioned_tables,
       count(*) FILTER (WHERE c.relrowsecurity) AS rls_enabled_tables,
       count(*) FILTER (WHERE c.relforcerowsecurity) AS rls_forced_tables
FROM pg_class AS c
JOIN pg_namespace AS n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind IN ('r', 'p');

\echo ''
\echo '=== 4. Policies: command, roles, USING and WITH CHECK ==='
SELECT schemaname,
       tablename,
       policyname,
       permissive,
       roles,
       cmd,
       qual AS using_expression,
       with_check AS with_check_expression
FROM pg_policies
WHERE schemaname = 'public'
ORDER BY tablename, policyname;

\echo ''
\echo '=== 5. Role attributes and membership edges ==='
SELECT r.rolname,
       r.rolcanlogin AS can_login,
       r.rolsuper AS superuser,
       r.rolcreatedb AS create_db,
       r.rolcreaterole AS create_role,
       r.rolinherit AS inherit_privileges,
       r.rolbypassrls AS bypass_rls,
       ARRAY(
           SELECT parent.rolname
           FROM pg_auth_members AS m
           JOIN pg_roles AS parent ON parent.oid = m.roleid
           WHERE m.member = r.oid
           ORDER BY parent.rolname
       ) AS granted_roles
FROM pg_roles AS r
WHERE r.rolcanlogin
   OR r.rolname IN (
       'stonez_api_runtime',
       'stonez_auth_bootstrap_owner',
       'stonez_auth_session_owner',
       'stonez_bulk_queue_owner',
       'anon',
       'authenticated',
       'service_role',
       'postgres'
   )
ORDER BY r.rolname;

\echo ''
\echo '=== 6. Effective table privileges for login and application roles ==='
\echo 'Privileges are evaluated with PostgreSQL has_table_privilege(), including inherited grants and ownership.'
SELECT n.nspname AS schema_name,
       c.relname AS table_name,
       grantee.rolname AS role_name,
       privilege.privilege_type,
       has_table_privilege(grantee.oid, c.oid, privilege.privilege_type) AS effective
FROM pg_class AS c
JOIN pg_namespace AS n ON n.oid = c.relnamespace
CROSS JOIN pg_roles AS grantee
CROSS JOIN (VALUES ('SELECT'), ('INSERT'), ('UPDATE'), ('DELETE'),
                   ('TRUNCATE'), ('REFERENCES'), ('TRIGGER')) AS privilege(privilege_type)
WHERE n.nspname = 'public'
  AND c.relkind IN ('r', 'p')
  AND (
      grantee.rolcanlogin
      OR grantee.rolname IN (
          'stonez_api_runtime',
          'stonez_auth_bootstrap_owner',
          'stonez_auth_session_owner',
          'stonez_bulk_queue_owner',
          'anon',
          'authenticated',
          'service_role',
          'postgres'
      )
  )
  AND has_table_privilege(grantee.oid, c.oid, privilege.privilege_type)
ORDER BY c.relname, grantee.rolname, privilege.privilege_type;

\echo ''
\echo '=== 7. Effective column-only privileges ==='
\echo 'Rows are limited to column privileges not already covered by a table-level grant.'
SELECT n.nspname AS schema_name,
       c.relname AS table_name,
       a.attname AS column_name,
       grantee.rolname AS role_name,
       privilege.privilege_type
FROM pg_class AS c
JOIN pg_namespace AS n ON n.oid = c.relnamespace
JOIN pg_attribute AS a ON a.attrelid = c.oid
CROSS JOIN pg_roles AS grantee
CROSS JOIN (VALUES ('SELECT'), ('INSERT'), ('UPDATE'), ('REFERENCES')) AS privilege(privilege_type)
WHERE n.nspname = 'public'
  AND c.relkind IN ('r', 'p')
  AND a.attnum > 0
  AND NOT a.attisdropped
  AND (
      grantee.rolcanlogin
      OR grantee.rolname IN (
          'stonez_api_runtime',
          'stonez_auth_bootstrap_owner',
          'stonez_auth_session_owner',
          'stonez_bulk_queue_owner',
          'anon',
          'authenticated',
          'service_role',
          'postgres'
      )
  )
  AND has_column_privilege(grantee.oid, c.oid, a.attnum, privilege.privilege_type)
  AND NOT has_table_privilege(grantee.oid, c.oid, privilege.privilege_type)
ORDER BY c.relname, a.attnum, grantee.rolname, privilege.privilege_type;

\echo ''
\echo '=== 8. Effective schema privileges on public ==='
SELECT r.rolname AS role_name,
       privilege.privilege_type,
       has_schema_privilege(r.oid, n.oid, privilege.privilege_type) AS effective
FROM pg_namespace AS n
CROSS JOIN pg_roles AS r
CROSS JOIN (VALUES ('USAGE'), ('CREATE')) AS privilege(privilege_type)
WHERE n.nspname = 'public'
  AND (
      r.rolcanlogin
      OR r.rolname IN (
          'stonez_api_runtime',
          'stonez_auth_bootstrap_owner',
          'stonez_auth_session_owner',
          'stonez_bulk_queue_owner',
          'anon',
          'authenticated',
          'service_role',
          'postgres'
      )
  )
  AND has_schema_privilege(r.oid, n.oid, privilege.privilege_type)
ORDER BY r.rolname, privilege.privilege_type;

\echo ''
\echo '=== 10. Explicit table ACL entries (grantor/grantee/options) ==='
SELECT n.nspname AS schema_name,
       c.relname AS table_name,
       grantor.rolname AS grantor,
       CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END AS grantee,
       acl.privilege_type,
       acl.is_grantable
FROM pg_class AS c
JOIN pg_namespace AS n ON n.oid = c.relnamespace
CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl, acldefault('r', c.relowner))) AS acl
LEFT JOIN pg_roles AS grantor ON grantor.oid = acl.grantor
LEFT JOIN pg_roles AS grantee ON grantee.oid = acl.grantee
WHERE n.nspname = 'public'
  AND c.relkind IN ('r', 'p')
ORDER BY c.relname, grantee, acl.privilege_type;

\echo ''
\echo '=== 11. RLS policy role resolution and permissive/restrictive counts ==='
SELECT p.schemaname,
       p.tablename,
       count(*) AS total_policies,
       count(*) FILTER (WHERE p.permissive = 'PERMISSIVE') AS permissive_policies,
       count(*) FILTER (WHERE p.permissive = 'RESTRICTIVE') AS restrictive_policies,
       count(*) FILTER (WHERE p.cmd IN ('ALL', 'INSERT', 'UPDATE', 'DELETE')) AS policies_relevant_to_writes
FROM pg_policies AS p
WHERE p.schemaname = 'public'
GROUP BY p.schemaname, p.tablename
ORDER BY p.tablename;

ROLLBACK;
\echo ''
\echo 'Audit complete. Transaction rolled back; database state was not changed.'
