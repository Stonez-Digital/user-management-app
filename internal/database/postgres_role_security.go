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
            sr.rolbypassrls AS session_bypass_rls
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
    if status.CurrentSuperuser || status.CurrentBypassRLS || status.SessionSuperuser || status.SessionBypassRLS {
        return fmt.Errorf(
            "REQUIRE_NOBYPASSRLS is enabled but PostgreSQL role current=%q (superuser=%t bypassrls=%t), session=%q (superuser=%t bypassrls=%t) is privileged",
            status.CurrentRole, status.CurrentSuperuser, status.CurrentBypassRLS,
            status.SessionRole, status.SessionSuperuser, status.SessionBypassRLS,
        )
    }
    return nil
}
