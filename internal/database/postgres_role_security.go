package database

import (
    "fmt"

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
    IsRuntimeMember bool `gorm:"column:is_runtime_member"`
    IsRuntimeEffective bool `gorm:"column:is_runtime_effective"`
    IsQueueOwnerMember bool `gorm:"column:is_queue_owner_member"`
    CanExecuteClaim bool `gorm:"column:can_execute_claim"`
    CurrentRoleMatchesSession bool `gorm:"column:current_role_matches_session"`
}

// ValidateBulkImportClaimRole checks that the worker's claim-only connection
// authenticates directly as a non-privileged login with effective membership
// in stonez_api_runtime and no queue-owner membership.
func ValidateBulkImportClaimRole(db *gorm.DB) error {
    if db == nil || db.Dialector.Name() != "postgres" {
        return fmt.Errorf("bulk import claim connection must be PostgreSQL")
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
            pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER') AS is_runtime_member,
            pg_has_role(session_user, 'stonez_api_runtime', 'USAGE') AS is_runtime_effective,
            pg_has_role(session_user, 'stonez_bulk_queue_owner', 'MEMBER') AS is_queue_owner_member,
            has_function_privilege(session_user, 'stonez_internal.claim_next_bulk_import_job(uuid)', 'EXECUTE') AS can_execute_claim,
            current_user = session_user AS current_role_matches_session
        FROM pg_roles AS role
        WHERE role.rolname = session_user
    `).Scan(&status).Error
    if err != nil {
        return fmt.Errorf("inspect bulk import claim role: %w", err)
    }
    if status.SessionRole == "" || !status.IsLogin || status.IsSuperuser || status.BypassRLS ||
        status.CanCreateDB || status.CanCreateRole || !status.IsRuntimeMember ||
        !status.IsRuntimeEffective || status.IsQueueOwnerMember || !status.CanExecuteClaim ||
        !status.CurrentRoleMatchesSession {
        return fmt.Errorf("bulk import claim connection role %q does not satisfy the restricted worker role boundary", status.SessionRole)
    }
    return nil
}
