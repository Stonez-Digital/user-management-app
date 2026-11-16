package database

import (
    "fmt"

    "gorm.io/gorm"
)

// BulkImportProcessingRoleStatus describes the authenticated processing
// connection. It contains no connection strings or secret material.
type BulkImportProcessingRoleStatus struct {
    CurrentRole string `gorm:"column:current_role"`
    SessionRole string `gorm:"column:session_role"`
    CurrentRoleMatchesSession bool `gorm:"column:current_role_matches_session"`
    SessionCanLogin bool `gorm:"column:session_can_login"`
    CurrentSuperuser bool `gorm:"column:current_superuser"`
    CurrentBypassRLS bool `gorm:"column:current_bypass_rls"`
    SessionSuperuser bool `gorm:"column:session_superuser"`
    SessionBypassRLS bool `gorm:"column:session_bypass_rls"`
    SessionCanCreateDB bool `gorm:"column:session_can_create_db"`
    SessionCanCreateRole bool `gorm:"column:session_can_create_role"`
    SessionCanAssumePrivilegedRole bool `gorm:"column:session_can_assume_privileged_role"`
    SessionIsAPIRuntimeMember bool `gorm:"column:session_is_api_runtime_member"`
    SessionIsQueueOwnerMember bool `gorm:"column:session_is_queue_owner_member"`
    SessionIsServiceRoleMember bool `gorm:"column:session_is_service_role_member"`
}

// InspectBulkImportProcessingRole performs a read-only inspection of the
// processing connection's authenticated identity and role memberships.
func InspectBulkImportProcessingRole(db *gorm.DB) (*BulkImportProcessingRoleStatus, error) {
    if db == nil || db.Dialector.Name() != "postgres" {
        return nil, fmt.Errorf("bulk import processing connection must be PostgreSQL")
    }
    var status BulkImportProcessingRoleStatus
    err := db.Raw(`
        SELECT current_user AS current_role,
               session_user AS session_role,
               current_user = session_user AS current_role_matches_session,
               sr.rolcanlogin AS session_can_login,
               cr.rolsuper AS current_superuser,
               cr.rolbypassrls AS current_bypass_rls,
               sr.rolsuper AS session_superuser,
               sr.rolbypassrls AS session_bypass_rls,
               sr.rolcreatedb AS session_can_create_db,
               sr.rolcreaterole AS session_can_create_role,
               pg_has_role(session_user, 'stonez_api_runtime', 'MEMBER') AS session_is_api_runtime_member,
               pg_has_role(session_user, 'stonez_bulk_queue_owner', 'MEMBER') AS session_is_queue_owner_member,
               pg_has_role(session_user, 'service_role', 'MEMBER') AS session_is_service_role_member,
               EXISTS (
                   SELECT 1 FROM pg_roles privileged
                    WHERE (privileged.rolsuper OR privileged.rolbypassrls)
                      AND privileged.rolname <> session_user
                      AND pg_has_role(session_user, privileged.oid, 'MEMBER')
               ) AS session_can_assume_privileged_role
          FROM pg_roles cr
          JOIN pg_roles sr ON sr.rolname = session_user
         WHERE cr.rolname = current_user
    `).Scan(&status).Error
    if err != nil {
        return nil, fmt.Errorf("inspect bulk import processing role: %w", err)
    }
    if status.SessionRole == "" {
        return nil, fmt.Errorf("inspect bulk import processing role: authenticated PostgreSQL role not found")
    }
    return &status, nil
}

// ValidateBulkImportProcessingRole is an opt-in, fail-closed startup identity
// guard. It does not prove table grants, RLS policy correctness, or tenant
// context integrity; those require isolated PostgreSQL integration tests.
func ValidateBulkImportProcessingRole(status *BulkImportProcessingRoleStatus, required bool) error {
    if !required {
        return nil
    }
    if status == nil {
        return fmt.Errorf("REQUIRE_BULK_IMPORT_PROCESS_ROLE requires a verifiable PostgreSQL processing role")
    }
    if status.SessionRole == "" || !status.SessionCanLogin ||
        status.CurrentRole != status.SessionRole || !status.CurrentRoleMatchesSession ||
        status.CurrentSuperuser || status.CurrentBypassRLS ||
        status.SessionSuperuser || status.SessionBypassRLS ||
        status.SessionCanCreateDB || status.SessionCanCreateRole ||
        status.SessionCanAssumePrivilegedRole ||
        status.SessionIsAPIRuntimeMember || status.SessionIsQueueOwnerMember ||
        status.SessionIsServiceRoleMember {
        return fmt.Errorf(
            "REQUIRE_BULK_IMPORT_PROCESS_ROLE is enabled but processing login %q does not satisfy the dedicated NOBYPASSRLS identity boundary",
            status.SessionRole,
        )
    }
    return nil
}
