package database

import (
	"strings"
	"testing"
)

func TestValidateBulkImportClaimRoleStatus(t *testing.T) {
	safe := &BulkImportClaimRoleStatus{
		SessionRole: "stonez_bulk_worker",
		IsLogin: true,
		IsClaimRoleMember: true,
		IsClaimRoleEffective: true,
		CanExecuteClaim: true,
		CurrentRoleMatchesSession: true,
	}

	tests := []struct {
		name string
		mutate func(*BulkImportClaimRoleStatus)
		wantErr bool
	}{
		{name: "dedicated claim login accepted"},
		{name: "missing status rejected", wantErr: true},
		{name: "login required", mutate: func(s *BulkImportClaimRoleStatus) { s.IsLogin = false }, wantErr: true},
		{name: "superuser rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsSuperuser = true }, wantErr: true},
		{name: "BYPASSRLS rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.BypassRLS = true }, wantErr: true},
		{name: "CREATEDB rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CanCreateDB = true }, wantErr: true},
		{name: "CREATEROLE rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CanCreateRole = true }, wantErr: true},
		{name: "claim membership required", mutate: func(s *BulkImportClaimRoleStatus) { s.IsClaimRoleMember = false }, wantErr: true},
		{name: "effective claim membership required", mutate: func(s *BulkImportClaimRoleStatus) { s.IsClaimRoleEffective = false }, wantErr: true},
		{name: "SET ROLE to claim group rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CanSetClaimRole = true }, wantErr: true},
		{name: "queue owner membership rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsQueueOwnerMember = true }, wantErr: true},
		{name: "API runtime membership rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsAPIRuntimeMember = true }, wantErr: true},
		{name: "direct queue privileges rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.HasDirectQueuePrivileges = true }, wantErr: true},
		{name: "direct tenant privileges rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.HasTenantTablePrivileges = true }, wantErr: true},
		{name: "claim function EXECUTE required", mutate: func(s *BulkImportClaimRoleStatus) { s.CanExecuteClaim = false }, wantErr: true},
		{name: "role switching rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CurrentRoleMatchesSession = false }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status *BulkImportClaimRoleStatus
			if tt.name != "missing status rejected" {
				copy := *safe
				status = &copy
				if tt.mutate != nil {
					tt.mutate(status)
				}
			}
			err := validateBulkImportClaimRoleStatus(status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBulkImportClaimRoleStatus() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "claim-only worker role boundary") {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
