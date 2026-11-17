package database

import (
	"strings"
	"testing"
)

func safeBulkImportClaimRole() *BulkImportClaimRoleStatus {
	return &BulkImportClaimRoleStatus{
		SessionRole: "stonez_bulk_claim_login",
		IsLogin: true,
		IsClaimGroupMember: true,
		IsClaimGroupEffective: true,
		CanExecuteClaim: true,
		CurrentRoleMatchesSession: true,
	}
}

func TestValidateBulkImportClaimRoleStatus(t *testing.T) {
	tests := []struct {
		name string
		mutate func(*BulkImportClaimRoleStatus)
		wantErr bool
	}{
		{name: "dedicated claim login accepted", mutate: func(*BulkImportClaimRoleStatus) {}},
		{name: "login required", mutate: func(s *BulkImportClaimRoleStatus) { s.IsLogin = false }, wantErr: true},
		{name: "superuser rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsSuperuser = true }, wantErr: true},
		{name: "bypassrls rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.BypassRLS = true }, wantErr: true},
		{name: "createdb rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CanCreateDB = true }, wantErr: true},
		{name: "createrole rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CanCreateRole = true }, wantErr: true},
		{name: "claim group membership required", mutate: func(s *BulkImportClaimRoleStatus) { s.IsClaimGroupMember = false }, wantErr: true},
		{name: "effective inherited claim group required", mutate: func(s *BulkImportClaimRoleStatus) { s.IsClaimGroupEffective = false }, wantErr: true},
		{name: "claim group cannot be assumed", mutate: func(s *BulkImportClaimRoleStatus) { s.CanSetClaimGroupRole = true }, wantErr: true},
		{name: "API runtime membership rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsAPIRuntimeMember = true }, wantErr: true},
		{name: "queue owner membership rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsQueueOwnerMember = true }, wantErr: true},
		{name: "service role membership rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.IsServiceRoleMember = true }, wantErr: true},
		{name: "privileged role membership rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.SessionCanAssumePrivilegedRole = true }, wantErr: true},
		{name: "claim function execute required", mutate: func(s *BulkImportClaimRoleStatus) { s.CanExecuteClaim = false }, wantErr: true},
		{name: "role switch rejected", mutate: func(s *BulkImportClaimRoleStatus) { s.CurrentRoleMatchesSession = false }, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := safeBulkImportClaimRole()
			tt.mutate(status)
			err := validateBulkImportClaimRoleStatus(status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBulkImportClaimRoleStatus() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "claim-only") {
				t.Fatalf("expected actionable claim-only error, got %v", err)
			}
		})
	}
}

func TestValidateBulkImportClaimRoleRequiresPostgres(t *testing.T) {
	if err := ValidateBulkImportClaimRole(nil); err == nil {
		t.Fatal("expected nil database to be rejected")
	}
}
