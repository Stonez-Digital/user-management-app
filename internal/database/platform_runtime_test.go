package database

import "testing"

func safePlatformRuntimeRole() *PlatformRuntimeRoleStatus {
	return &PlatformRuntimeRoleStatus{CurrentRole: PlatformRuntimeRoleName, SessionRole: PlatformRuntimeRoleName, CanLogin: true}
}

func TestValidatePlatformRuntimeRole(t *testing.T) {
	tests := []struct {
		name string
		mutate func(*PlatformRuntimeRoleStatus)
		wantErr bool
	}{
		{name: "dedicated restricted role accepted", mutate: func(*PlatformRuntimeRoleStatus) {}},
		{name: "missing status rejected", wantErr: true},
		{name: "unexpected login rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.SessionRole = "postgres" }, wantErr: true},
		{name: "effective role mismatch rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.CurrentRole = "postgres" }, wantErr: true},
		{name: "no login rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.CanLogin = false }, wantErr: true},
		{name: "superuser rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.Superuser = true }, wantErr: true},
		{name: "bypassrls rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.BypassRLS = true }, wantErr: true},
		{name: "createdb rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.CanCreateDB = true }, wantErr: true},
		{name: "createrole rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.CanCreateRole = true }, wantErr: true},
		{name: "replication rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.Replication = true }, wantErr: true},
		{name: "inherit rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.Inherit = true }, wantErr: true},
		{name: "role membership rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.HasRoleMemberships = true }, wantErr: true},
		{name: "public relation ownership rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.OwnsPublicRelations = true }, wantErr: true},
		{name: "public schema create rejected", mutate: func(s *PlatformRuntimeRoleStatus) { s.CanCreateInPublicSchema = true }, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mutate == nil {
				if err := ValidatePlatformRuntimeRole(nil); (err != nil) != tt.wantErr { t.Fatalf("error = %v, wantErr %t", err, tt.wantErr) }
				return
			}
			status := safePlatformRuntimeRole()
			tt.mutate(status)
			if err := ValidatePlatformRuntimeRole(status); (err != nil) != tt.wantErr { t.Fatalf("error = %v, wantErr %t", err, tt.wantErr) }
		})
	}
}

func TestValidatePlatformDatabaseConfiguration(t *testing.T) {
	tests := []struct {
		name, driver, dsn string
		required bool
		wantErr bool
	}{
		{name: "legacy mode may omit platform connection", driver: "postgres"},
		{name: "required mode needs postgres", driver: "sqlite", required: true, wantErr: true},
		{name: "required mode needs dedicated DSN", driver: "postgres", required: true, wantErr: true},
		{name: "configured DSN requires postgres", driver: "sqlite", dsn: "postgres://example", wantErr: true},
		{name: "configured postgres capability accepted", driver: "postgres", dsn: "postgres://example", required: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePlatformDatabaseConfiguration(tt.required, tt.driver, tt.dsn); (err != nil) != tt.wantErr { t.Fatalf("error = %v, wantErr %t", err, tt.wantErr) }
		})
	}
}
