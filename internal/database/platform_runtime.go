package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const PlatformRuntimeRoleName = "stonez_platform_runtime"

type PlatformRuntimeRoleStatus struct {
	CurrentRole string
	SessionRole string
	CanLogin bool
	Superuser bool
	BypassRLS bool
	CanCreateDB bool
	CanCreateRole bool
	Replication bool
	Inherit bool
	HasRoleMemberships bool
	OwnsPublicRelations bool
	CanCreateInPublicSchema bool
}

func ValidatePlatformRuntimeRole(status *PlatformRuntimeRoleStatus) error {
	if status == nil {
		return fmt.Errorf("platform database capability could not be inspected")
	}
	if status.CurrentRole != PlatformRuntimeRoleName || status.SessionRole != PlatformRuntimeRoleName ||
		status.CurrentRole != status.SessionRole || !status.CanLogin ||
		status.Superuser || status.BypassRLS || status.CanCreateDB ||
		status.CanCreateRole || status.Replication || status.Inherit ||
		status.HasRoleMemberships || status.OwnsPublicRelations ||
		status.CanCreateInPublicSchema {
		return fmt.Errorf(
			"platform database login must be %q with LOGIN, NOBYPASSRLS, NOINHERIT, no role memberships, no public relation ownership, and no schema CREATE privilege (current=%q session=%q login=%t superuser=%t bypassrls=%t createdb=%t createrole=%t replication=%t inherit=%t memberships=%t owns_relations=%t public_create=%t)",
			PlatformRuntimeRoleName, status.CurrentRole, status.SessionRole,
			status.CanLogin, status.Superuser, status.BypassRLS, status.CanCreateDB,
			status.CanCreateRole, status.Replication, status.Inherit,
			status.HasRoleMemberships, status.OwnsPublicRelations,
			status.CanCreateInPublicSchema,
		)
	}
	return nil
}

func ValidatePlatformDatabaseConfiguration(required bool, driver, dsn string) error {
	driver = strings.ToLower(strings.TrimSpace(driver))
	dsn = strings.TrimSpace(dsn)
	if required && driver != "postgres" {
		return fmt.Errorf("REQUIRE_PLATFORM_DATABASE=true requires DB_DRIVER=postgres")
	}
	if required && dsn == "" {
		return fmt.Errorf("REQUIRE_PLATFORM_DATABASE=true requires PLATFORM_DATABASE_URL")
	}
	if dsn != "" && driver != "postgres" {
		return fmt.Errorf("PLATFORM_DATABASE_URL requires DB_DRIVER=postgres")
	}
	return nil
}

// ConnectPlatformRuntime opens a separate platform-only pool. A nil result
// means legacy mode is explicitly permitted because the required flag is off.
func ConnectPlatformRuntime(required bool) (*gorm.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("PLATFORM_DATABASE_URL"))
	driver := strings.TrimSpace(os.Getenv("DB_DRIVER"))
	if err := ValidatePlatformDatabaseConfiguration(required, driver, dsn); err != nil {
		return nil, err
	}
	if dsn == "" {
		return nil, nil
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open platform runtime database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get platform runtime connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping platform runtime database: %w", err)
	}

	var status PlatformRuntimeRoleStatus
	err = db.WithContext(ctx).Raw("SELECT current_user AS current_role, session_user AS session_role, r.rolcanlogin AS can_login, r.rolsuper AS superuser, r.rolbypassrls AS bypass_rls, r.rolcreatedb AS can_create_db, r.rolcreaterole AS can_create_role, r.rolreplication AS replication, r.rolinherit AS inherit, EXISTS (SELECT 1 FROM pg_auth_members m WHERE m.member = r.oid) AS has_role_memberships, EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'public' AND c.relkind IN ('r','p','v','m','f','S') AND c.relowner = r.oid) AS owns_public_relations, has_schema_privilege(session_user, 'public', 'CREATE') AS can_create_in_public_schema FROM pg_roles r WHERE r.rolname = session_user").Scan(&status).Error
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("inspect platform runtime database role: %w", err)
	}
	if err := ValidatePlatformRuntimeRole(&status); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("platform runtime database role is not restricted: %w", err)
	}
	return db, nil
}
