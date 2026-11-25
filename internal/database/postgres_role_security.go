package database

import (
    "fmt"
    "strings"

    "gorm.io/gorm"
)

// PostgresRoleSecurityStatus reports the effective PostgreSQL role and the
// role that authenticated the connection. It intentionally excludes secrets
// and connection details.
type PostgresRoleSecurityStatus struct {
    CurrentRole      string `gorm:"column:current_role"`
    SessionRole      string `gorm:"column:session_role"`
    CurrentSuperuser bool   `gorm:"column:current_superuser"`
    CurrentBypassRLS bool   `gorm:"column:current_bypass_rls"`
    SessionSuperuser bool   `gorm:"column:session_superuser"`
    SessionBypassRLS bool   `gorm:"column:session_bypass_rls"`
    SessionCanAssumePrivilegedRole bool `gorm:"column:session_can_assume_privileged_role"`
}

// InspectPostgresRole performs a read-only privilege inspection. SQLite
// deployments are intentionally skipped. No role grants or database state
// are changed.
func InspectPostgresRole(db *gorm.DB) (*PostgresRoleSecurityStatus, error) {
    if db == nil || db.Dialector.Name() != "postgres" {
        return nil, nil
    }

    var status PostgresRoleSecurityStatus
    err := db.Raw(`
        SELECT
            current_user AS current_role,
            session_user AS session_role,
            cr.rolsuper AS current_superuser,
            cr.rolbypassrls AS current_bypass_rls,
            sr.rolsuper AS session_superuser,
            sr.rolbypassrls AS session_bypass_rls,
            EXISTS (
                SELECT 1
                FROM pg_roles AS privileged
                WHERE (privileged.rolsuper OR privileged.rolbypassrls)
                  AND privileged.rolname <> session_user
                  AND pg_has_role(session_user, privileged.oid, 'MEMBER')
            ) AS session_can_assume_privileged_role
        FROM pg_roles AS cr
        JOIN pg_roles AS sr ON sr.rolname = session_user
        WHERE cr.rolname = current_user
    `).Scan(&status).Error
    if err != nil {
        return nil, err
    }
    return &status, nil
}

// ValidatePostgresRoleSecurity enforces the optional restricted-runtime-role
// startup gate. It does not mutate PostgreSQL roles or privileges.
func ValidatePostgresRoleSecurity(status *PostgresRoleSecurityStatus, required bool) error {
    if !required {
        return nil
    }
    if status == nil {
        return fmt.Errorf("REQUIRE_NOBYPASSRLS requires a verifiable PostgreSQL runtime role")
    }
    if status.CurrentSuperuser || status.CurrentBypassRLS || status.SessionSuperuser || status.SessionBypassRLS || status.SessionCanAssumePrivilegedRole {
        return fmt.Errorf(
            "REQUIRE_NOBYPASSRLS is enabled but PostgreSQL role current=%q (superuser=%t bypassrls=%t), session=%q (superuser=%t bypassrls=%t can_assume_privileged_role=%t) is privileged",
            status.CurrentRole, status.CurrentSuperuser, status.CurrentBypassRLS,
            status.SessionRole, status.SessionSuperuser, status.SessionBypassRLS, status.SessionCanAssumePrivilegedRole,
        )
    }
    return nil
}


type BulkImportClaimRoleStatus struct {
    SessionRole string `gorm:"column:session_role"`
    IsLogin bool `gorm:"column:is_login"`
    IsSuperuser bool `gorm:"column:is_superuser"`
    BypassRLS bool `gorm:"column:bypass_rls"`
    CanCreateDB bool `gorm:"column:can_create_db"`
    CanCreateRole bool `gorm:"column:can_create_role"`
    IsClaimRoleMember bool `gorm:"column:is_claim_role_member"`
    IsClaimRoleEffective bool `gorm:"column:is_claim_role_effective"`
	CanSetClaimRole bool `gorm:"column:can_set_claim_role"`
    IsQueueOwnerMember bool `gorm:"column:is_queue_owner_member"`
    IsAPIRuntimeMember bool `gorm:"column:is_api_runtime_member"`
    HasDirectQueuePrivileges bool `gorm:"column:has_direct_queue_privileges"`
    HasTenantTablePrivileges bool `gorm:"column:has_tenant_table_privileges"`
    CanExecuteClaim bool `gorm:"column:can_execute_claim"`
    CurrentRoleMatchesSession bool `gorm:"column:current_role_matches_session"`
}

// ValidateBulkImportClaimRole checks that the worker's claim-only connection
// authenticates directly as a non-privileged login with effective membership
// in stonez_bulk_queue_claim only, with no direct table access or API/queue-owner membership.
func ValidateBulkImportClaimRole(db *gorm.DB) error {
    return ValidateBulkImportClaimRoleFor(
        db,
        "stonez_bulk_queue_claim",
        "stonez_bulk_queue_owner",
        "stonez_internal.claim_next_bulk_import_job(uuid)",
    )
}

// ValidateBulkImportClaimRoleFor is also used by isolated PostgreSQL fixtures,
// which substitute unique schema and role names to avoid collisions.
func ValidateBulkImportClaimRoleFor(db *gorm.DB, claimRole, queueOwnerRole, claimFunction string) error {
    if db == nil || db.Dialector.Name() != "postgres" {
        return fmt.Errorf("bulk import claim connection must be PostgreSQL")
    }
    if strings.TrimSpace(claimRole) == "" || strings.TrimSpace(queueOwnerRole) == "" || strings.TrimSpace(claimFunction) == "" {
        return fmt.Errorf("bulk import claim role names and function signature are required")
    }
    var status BulkImportClaimRoleStatus
    err := db.Raw(`
        SELECT
            session_user AS session_role,
            role.rolcanlogin AS is_login,
            role.rolsuper AS is_superuser,
            role.rolbypassrls AS bypass_rls,
            role.rolcreatedb AS can_create_db,
            role.rolcreaterole AS can_create_role,
            pg_has_role(session_user, ?::name, 'MEMBER') AS is_claim_role_member,
            pg_has_role(session_user, ?::name, 'USAGE') AS is_claim_role_effective,
            pg_has_role(session_user, ?::name, 'SET') AS can_set_claim_role,
            pg_has_role(session_user, ?::name, 'MEMBER') AS is_queue_owner_member,
            pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER') AS is_api_runtime_member,
            (has_table_privilege(session_user, 'public.bulk_import_jobs', 'SELECT') OR
             has_table_privilege(session_user, 'public.bulk_import_jobs', 'INSERT') OR
             has_table_privilege(session_user, 'public.bulk_import_jobs', 'UPDATE') OR
             has_table_privilege(session_user, 'public.bulk_import_jobs', 'DELETE') OR
             has_table_privilege(session_user, 'public.bulk_import_jobs', 'TRUNCATE') OR
             has_any_column_privilege(session_user, 'public.bulk_import_jobs', 'SELECT') OR
             has_any_column_privilege(session_user, 'public.bulk_import_jobs', 'INSERT') OR
             has_any_column_privilege(session_user, 'public.bulk_import_jobs', 'UPDATE')) AS has_direct_queue_privileges,
            EXISTS (
                SELECT 1
                  FROM pg_class AS tenant_relation
                  JOIN pg_namespace AS tenant_schema ON tenant_schema.oid = tenant_relation.relnamespace
                 WHERE tenant_schema.nspname = 'public'
                   AND tenant_relation.relkind IN ('r', 'p', 'v', 'm', 'f')
                   AND tenant_relation.relname <> 'bulk_import_jobs'
                   AND (
                       has_table_privilege(session_user, tenant_relation.oid, 'SELECT') OR
                       has_table_privilege(session_user, tenant_relation.oid, 'INSERT') OR
                       has_table_privilege(session_user, tenant_relation.oid, 'UPDATE') OR
                       has_table_privilege(session_user, tenant_relation.oid, 'DELETE') OR
                       has_table_privilege(session_user, tenant_relation.oid, 'TRUNCATE') OR
                       has_table_privilege(session_user, tenant_relation.oid, 'REFERENCES') OR
                       has_table_privilege(session_user, tenant_relation.oid, 'TRIGGER') OR
                       has_any_column_privilege(session_user, tenant_relation.oid, 'SELECT') OR
                       has_any_column_privilege(session_user, tenant_relation.oid, 'INSERT') OR
                       has_any_column_privilege(session_user, tenant_relation.oid, 'UPDATE') OR
                       has_any_column_privilege(session_user, tenant_relation.oid, 'REFERENCES')
                   )
            ) AS has_tenant_table_privileges,
            has_function_privilege(session_user, ?::regprocedure, 'EXECUTE') AS can_execute_claim,
            current_user = session_user AS current_role_matches_session
        FROM pg_roles AS role
        WHERE role.rolname = session_user
    `, claimRole, claimRole, claimRole, queueOwnerRole, claimFunction).Scan(&status).Error
    if err != nil {
        return fmt.Errorf("inspect bulk import claim role: %w", err)
    }
    return validateBulkImportClaimRoleStatus(&status)
}


// validateBulkImportClaimRoleStatus applies the fail-closed startup assertions
// after the connection's PostgreSQL catalog state has been inspected.
func validateBulkImportClaimRoleStatus(status *BulkImportClaimRoleStatus) error {
    if status == nil || status.SessionRole == "" || !status.IsLogin ||
        status.IsSuperuser || status.BypassRLS || status.CanCreateDB || status.CanCreateRole ||
        !status.IsClaimRoleMember || !status.IsClaimRoleEffective || status.CanSetClaimRole ||
        status.IsQueueOwnerMember || status.IsAPIRuntimeMember ||
        status.HasDirectQueuePrivileges || status.HasTenantTablePrivileges ||
        !status.CanExecuteClaim || !status.CurrentRoleMatchesSession {
        role := "<unavailable>"
        if status != nil && status.SessionRole != "" {
            role = status.SessionRole
        }
        return fmt.Errorf("bulk import claim connection role %q does not satisfy the claim-only worker role boundary", role)
    }
    return nil
}
