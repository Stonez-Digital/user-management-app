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
    }
    tests := []struct {
        name string
        status *BulkImportProcessingRoleStatus
        required bool
        wantErr bool
    }{
        {name: "guard disabled", status: nil, required: false},
        {name: "guard enabled without inspection", status: nil, required: true, wantErr: true},
        {name: "dedicated non privileged login accepted", status: safe, required: true},
        {name: "login missing", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", CurrentRoleMatchesSession: true}, required: true, wantErr: true},
        {name: "role switch detected", status: &BulkImportProcessingRoleStatus{CurrentRole: "other", SessionRole: "worker", SessionCanLogin: true}, required: true, wantErr: true},
        {name: "superuser rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionSuperuser: true}, required: true, wantErr: true},
        {name: "bypassrls rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionBypassRLS: true}, required: true, wantErr: true},
        {name: "createdb rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionCanCreateDB: true}, required: true, wantErr: true},
        {name: "createrole rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionCanCreateRole: true}, required: true, wantErr: true},
        {name: "privileged membership rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionCanAssumePrivilegedRole: true}, required: true, wantErr: true},
        {name: "API runtime membership rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionIsAPIRuntimeMember: true}, required: true, wantErr: true},
        {name: "queue owner membership rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionIsQueueOwnerMember: true}, required: true, wantErr: true},
        {name: "service role membership rejected", status: &BulkImportProcessingRoleStatus{CurrentRole: "worker", SessionRole: "worker", SessionCanLogin: true, CurrentRoleMatchesSession: true, SessionIsServiceRoleMember: true}, required: true, wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateBulkImportProcessingRole(tt.status, tt.required)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ValidateBulkImportProcessingRole() error = %v, wantErr %t", err, tt.wantErr)
            }
            if err != nil && tt.required && tt.status != nil && !strings.Contains(err.Error(), "REQUIRE_BULK_IMPORT_PROCESS_ROLE") {
                t.Fatalf("expected actionable guard error, got %v", err)
            }
        })
    }
}
