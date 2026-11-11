package database

import "gorm.io/gorm"

// PostgresRoleSecurityStatus reports the effective PostgreSQL role and the
// role that authenticated the connection. It intentionally excludes secrets
// and connection details.
type PostgresRoleSecurityStatus struct {
    CurrentRole       string `gorm:"column:current_role"`
    SessionRole       string `gorm:"column:session_role"`
    CurrentSuperuser  bool   `gorm:"column:current_superuser"`
    CurrentBypassRLS  bool   `gorm:"column:current_bypass_rls"`
    SessionSuperuser  bool   `gorm:"column:session_superuser"`
    SessionBypassRLS  bool   `gorm:"column:session_bypass_rls"`
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
            current_role.rolsuper AS current_superuser,
            current_role.rolbypassrls AS current_bypass_rls,
            session_role.rolsuper AS session_superuser,
            session_role.rolbypassrls AS session_bypass_rls
        FROM pg_roles AS current_role
        JOIN pg_roles AS session_role ON session_role.rolname = session_user
        WHERE current_role.rolname = current_user
    `).Scan(&status).Error
    if err != nil {
        return nil, err
    }
    return &status, nil
}
