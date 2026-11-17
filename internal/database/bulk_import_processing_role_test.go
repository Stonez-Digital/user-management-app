package database

import (
	"strings"
	"testing"
)

func TestValidateBulkImportProcessingRole(t *testing.T) {
	safe := &BulkImportProcessingRoleStatus{
		CurrentRole: "stonez_bulk_process_login",
		SessionRole: "stonez_bulk_process_login",
		CurrentRoleMatchesSession: true,
		SessionCanLogin: true,
		SessionIsProcessingGroupMember: true,
	}
	tests := []struct {
		name string
		mutate func(*BulkImportProcessingRoleStatus)
		wantErr bool
	}{
		{name: "dedicated restricted login accepted"},
		{name: "missing status rejected", mutate: nil, wantErr: true},
		{name: "role switch rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.CurrentRole = "other" }, wantErr: true},
		{name: "superuser rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionSuperuser = true }, wantErr: true},
		{name: "bypassrls rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionBypassRLS = true }, wantErr: true},
		{name: "createdb rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionCanCreateDB = true }, wantErr: true},
		{name: "createrole rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionCanCreateRole = true }, wantErr: true},
		{name: "privileged membership rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionCanAssumePrivilegedRole = true }, wantErr: true},
		{name: "API runtime membership rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionIsAPIRuntimeMember = true }, wantErr: true},
		{name: "queue owner membership rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionIsQueueOwnerMember = true }, wantErr: true},
		{name: "service role membership rejected", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionIsServiceRoleMember = true }, wantErr: true},
		{name: "processing group membership required", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionIsProcessingGroupMember = false }, wantErr: true},
		{name: "login required", mutate: func(s *BulkImportProcessingRoleStatus) { s.SessionCanLogin = false }, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status *BulkImportProcessingRoleStatus
			if tt.name != "missing status rejected" {
				copy := *safe
				status = &copy
				if tt.mutate != nil { tt.mutate(status) }
			}
			err := ValidateBulkImportProcessingRole(status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateBulkImportProcessingRole() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "dedicated NOBYPASSRLS processing-group boundary") {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
