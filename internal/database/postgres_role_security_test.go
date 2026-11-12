package database

import (
    "strings"
    "testing"
)

func TestValidatePostgresRoleSecurity(t *testing.T) {
    safe := &PostgresRoleSecurityStatus{
        CurrentRole: "stonez_api", SessionRole: "stonez_api",
    }
    tests := []struct {
        name string
        status *PostgresRoleSecurityStatus
        required bool
        wantErr bool
    }{
        {name: "guard disabled", status: nil, required: false, wantErr: false},
        {name: "guard enabled without postgres inspection", status: nil, required: true, wantErr: true},
        {name: "restricted role accepted", status: safe, required: true, wantErr: false},
        {name: "effective role bypass rejected", status: &PostgresRoleSecurityStatus{CurrentRole: "postgres", SessionRole: "postgres", CurrentBypassRLS: true}, required: true, wantErr: true},
        {name: "session role bypass rejected", status: &PostgresRoleSecurityStatus{CurrentRole: "stonez_api", SessionRole: "postgres", SessionBypassRLS: true}, required: true, wantErr: true},
        {name: "superuser rejected", status: &PostgresRoleSecurityStatus{CurrentRole: "root", SessionRole: "root", CurrentSuperuser: true}, required: true, wantErr: true},
        {name: "privileged role membership rejected", status: &PostgresRoleSecurityStatus{CurrentRole: "stonez_api", SessionRole: "stonez_api", SessionCanAssumePrivilegedRole: true}, required: true, wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePostgresRoleSecurity(tt.status, tt.required)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ValidatePostgresRoleSecurity() error = %v, wantErr %t", err, tt.wantErr)
            }
            if err != nil && tt.required && tt.status != nil && !strings.Contains(err.Error(), "REQUIRE_NOBYPASSRLS") {
                t.Fatalf("expected actionable guard error, got %v", err)
            }
        })
    }
}
