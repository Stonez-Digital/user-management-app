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
    CanReplicate bool `gorm:"column:can_replicate"`
    Inherits bool `gorm:"column:inherits"`
    IsClaimMember bool `gorm:"column:is_claim_member"`
    IsClaimEffective bool `gorm:"column:is_claim_effective"`
    CanSetClaimRole bool `gorm:"column:can_set_claim_role"`
    HasInternalSchemaUsage bool `gorm:"column:has_internal_schema_usage"`
    CanCreateInternalSchema bool `gorm:"column:can_create_internal_schema"`
    IsAPIRuntimeMember bool `gorm:"column:is_api_runtime_member"`
    IsQueueOwnerMember bool `gorm:"column:is_queue_owner_member"`
    CanExecuteClaim bool `gorm:"column:can_execute_claim"`
    HasPublicTablePrivileges bool `gorm:"column:has_public_table_privileges"`
    HasPublicSequencePrivileges bool `gorm:"column:has_public_sequence_privileges"`
    HasUnexpectedMembership bool `gorm:"column:has_unexpected_membership"`
    CurrentRoleMatchesSession bool `gorm:"column:current_role_matches_session"`
}

// ValidateBulkImportClaimRole checks that the claim-only connection uses a
// dedicated login that inherits only the queue-claim group. It must not inherit
// the broad API runtime, owner roles, or direct public-table privileges.
func ValidateBulkImportClaimRole(db *gorm.DB) error {
    return ValidateBulkImportClaimRoleFor(
        db,
        "stonez_bulk_queue_claim",
        "stonez_api_runtime",
        "stonez_bulk_queue_owner",
        "public",
        "stonez_internal",
        "stonez_internal.claim_next_bulk_import_job(uuid)",
    )
}

// ValidateBulkImportClaimRoleFor is also used by isolated PostgreSQL fixtures,
// which substitute unique schema and role names to avoid collisions.
func ValidateBulkImportClaimRoleFor(db *gorm.DB, claimRole, apiRuntimeRole, queueOwnerRole, publicSchema, internalSchema, claimFunction string) error {
    if db == nil || db.Dialector.Name() != "postgres" {
        return fmt.Errorf("bulk import claim connection must be PostgreSQL")
    }
    if strings.TrimSpace(claimRole) == "" || strings.TrimSpace(apiRuntimeRole) == "" ||
        strings.TrimSpace(queueOwnerRole) == "" || strings.TrimSpace(publicSchema) == "" ||
        strings.TrimSpace(internalSchema) == "" || strings.TrimSpace(claimFunction) == "" {
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
            role.rolreplication AS can_replicate,
            role.rolinherit AS inherits,
            pg_has_role(session_user, ?::name, 'MEMBER') AS is_claim_member,
            pg_has_role(session_user, ?::name, 'USAGE') AS is_claim_effective,
            pg_has_role(session_user, ?::name, 'SET') AS can_set_claim_role,
            pg_has_role(session_user, ?::name, 'MEMBER') AS is_api_runtime_member,
            pg_has_role(session_user, ?::name, 'MEMBER') AS is_queue_owner_member,
            has_function_privilege(session_user, ?::regprocedure, 'EXECUTE') AS can_execute_claim,
            has_schema_privilege(session_user, ?::name, 'USAGE') AS has_internal_schema_usage,
            has_schema_privilege(session_user, ?::name, 'CREATE') AS can_create_internal_schema,
            EXISTS (
                SELECT 1 FROM pg_class AS relation
                JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
                WHERE namespace.nspname = ?::name
                  AND relation.relkind IN ('r','p','v','m')
                  AND (has_table_privilege(session_user, relation.oid, 'SELECT')
                    OR has_table_privilege(session_user, relation.oid, 'INSERT')
                    OR has_table_privilege(session_user, relation.oid, 'UPDATE')
                    OR has_table_privilege(session_user, relation.oid, 'DELETE')
                    OR has_table_privilege(session_user, relation.oid, 'TRUNCATE')
                    OR has_table_privilege(session_user, relation.oid, 'REFERENCES')
                    OR has_table_privilege(session_user, relation.oid, 'TRIGGER'))
            ) AS has_public_table_privileges,
            EXISTS (
                SELECT 1 FROM pg_class AS relation
                JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
                WHERE namespace.nspname = ?::name
                  AND relation.relkind = 'S'
                  AND (has_sequence_privilege(session_user, relation.oid, 'USAGE')
                    OR has_sequence_privilege(session_user, relation.oid, 'SELECT')
                    OR has_sequence_privilege(session_user, relation.oid, 'UPDATE'))
            ) AS has_public_sequence_privileges,
            EXISTS (
                SELECT 1 FROM pg_auth_members AS membership
                 WHERE membership.member = role.oid
                   AND membership.roleid <> (SELECT oid FROM pg_roles WHERE rolname = ?::name)
            ) AS has_unexpected_membership,
            current_user = session_user AS current_role_matches_session
        FROM pg_roles AS role
        WHERE role.rolname = session_user
    `, claimRole, claimRole, claimRole, apiRuntimeRole, queueOwnerRole, claimFunction,
        internalSchema, internalSchema, publicSchema, publicSchema, claimRole).Scan(&status).Error
    if err != nil {
        return fmt.Errorf("inspect bulk import claim role: %w", err)
    }
    if status.SessionRole == "" || !status.IsLogin || status.IsSuperuser || status.BypassRLS ||
        status.CanCreateDB || status.CanCreateRole || status.CanReplicate || !status.Inherits ||
        !status.IsClaimMember || !status.IsClaimEffective || status.CanSetClaimRole ||
        status.IsAPIRuntimeMember || status.IsQueueOwnerMember || !status.CanExecuteClaim ||
        !status.HasInternalSchemaUsage || status.CanCreateInternalSchema || status.HasPublicTablePrivileges ||
        status.HasPublicSequencePrivileges || status.HasUnexpectedMembership ||
        !status.CurrentRoleMatchesSession {
        return fmt.Errorf("bulk import claim connection role %q does not satisfy the dedicated queue-claim boundary", status.SessionRole)
    }
    return nil
}
