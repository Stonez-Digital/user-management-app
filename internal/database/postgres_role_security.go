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
	IsClaimGroupMember bool `gorm:"column:is_claim_group_member"`
	IsClaimGroupEffective bool `gorm:"column:is_claim_group_effective"`
	CanSetClaimGroupRole bool `gorm:"column:can_set_claim_group_role"`
	IsAPIRuntimeMember bool `gorm:"column:is_api_runtime_member"`
	IsQueueOwnerMember bool `gorm:"column:is_queue_owner_member"`
	IsServiceRoleMember bool `gorm:"column:is_service_role_member"`
	HasUnexpectedRoleMembership bool `gorm:"column:has_unexpected_role_membership"`
	SessionCanAssumePrivilegedRole bool `gorm:"column:session_can_assume_privileged_role"`
	CanExecuteClaim bool `gorm:"column:can_execute_claim"`
	CurrentRoleMatchesSession bool `gorm:"column:current_role_matches_session"`
}

// ValidateBulkImportClaimRole requires a dedicated claim-only login. The
// authenticated login must inherit the claim group without being able to SET
// ROLE into it, and must not inherit API, queue-owner, service, or privileged roles.
func ValidateBulkImportClaimRole(db *gorm.DB) error {
	return ValidateBulkImportClaimRoleFor(
		db,
		"stonez_bulk_claim_group",
		"stonez_api_runtime",
		"stonez_bulk_queue_owner",
		"stonez_internal.claim_next_bulk_import_job(uuid)",
	)
}

// ValidateBulkImportClaimRoleFor supports isolated PostgreSQL fixtures with
// unique role names while enforcing the same claim-only capability boundary.
func ValidateBulkImportClaimRoleFor(db *gorm.DB, claimGroupRole, apiRuntimeRole, queueOwnerRole, claimFunction string) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("bulk import claim connection must be PostgreSQL")
	}
	if strings.TrimSpace(claimGroupRole) == "" || strings.TrimSpace(apiRuntimeRole) == "" ||
		strings.TrimSpace(queueOwnerRole) == "" || strings.TrimSpace(claimFunction) == "" {
		return fmt.Errorf("bulk import claim role names and function signature are required")
	}
	var status BulkImportClaimRoleStatus
	err := db.Raw(`
		SELECT
			session_user AS session_role,
			session_role.rolcanlogin AS is_login,
			session_role.rolsuper AS is_superuser,
			session_role.rolbypassrls AS bypass_rls,
			session_role.rolcreatedb AS can_create_db,
			session_role.rolcreaterole AS can_create_role,
			pg_has_role(session_user, ?::name, 'MEMBER') AS is_claim_group_member,
			pg_has_role(session_user, ?::name, 'USAGE') AS is_claim_group_effective,
			pg_has_role(session_user, ?::name, 'SET') AS can_set_claim_group_role,
			pg_has_role(session_user, ?::name, 'MEMBER') AS is_api_runtime_member,
			pg_has_role(session_user, ?::name, 'MEMBER') AS is_queue_owner_member,
			COALESCE((
				SELECT pg_has_role(session_user, service_role.oid, 'MEMBER')
				FROM pg_roles AS service_role
				WHERE service_role.rolname = 'service_role'
			), false) AS is_service_role_member,
			EXISTS (
				SELECT 1 FROM pg_auth_members AS membership
				WHERE membership.member = session_role.oid
				  AND membership.roleid <> (SELECT oid FROM pg_roles WHERE rolname = ?)
			) AS has_unexpected_role_membership,
			EXISTS (
				SELECT 1
				FROM pg_roles AS privileged
				WHERE (privileged.rolsuper OR privileged.rolbypassrls)
				  AND privileged.rolname <> session_user
				  AND pg_has_role(session_user, privileged.oid, 'MEMBER')
			) AS session_can_assume_privileged_role,
			has_function_privilege(session_user, ?::regprocedure, 'EXECUTE') AS can_execute_claim,
			current_user = session_user AS current_role_matches_session
		FROM pg_roles AS session_role
		WHERE session_role.rolname = session_user
	`, claimGroupRole, claimGroupRole, claimGroupRole, apiRuntimeRole, queueOwnerRole, claimGroupRole, claimFunction).Scan(&status).Error
	if err != nil {
		return fmt.Errorf("inspect bulk import claim role: %w", err)
	}
	return validateBulkImportClaimRoleStatus(&status)
}

func validateBulkImportClaimRoleStatus(status *BulkImportClaimRoleStatus) error {
	if status == nil || status.SessionRole == "" || !status.IsLogin || status.IsSuperuser || status.BypassRLS ||
		status.CanCreateDB || status.CanCreateRole ||
		!status.IsClaimGroupMember || !status.IsClaimGroupEffective || status.CanSetClaimGroupRole ||
		status.IsAPIRuntimeMember || status.IsQueueOwnerMember || status.IsServiceRoleMember || status.HasUnexpectedRoleMembership ||
		status.SessionCanAssumePrivilegedRole || !status.CanExecuteClaim || !status.CurrentRoleMatchesSession {
		role := ""
		if status != nil {
			role = status.SessionRole
		}
		return fmt.Errorf("bulk import claim connection role %q does not satisfy the dedicated claim-only login boundary", role)
	}
	return nil
}
