package main

import "testing"

func TestParseOptionalBoolEnv(t *testing.T) {
    tests := []struct {
        name string
        value string
        want bool
        wantErr bool
    }{
        {name: "unset defaults false", value: "", want: false},
        {name: "true", value: "true", want: true},
        {name: "false", value: "false", want: false},
        {name: "invalid rejected", value: "enabled", wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Setenv("TEST_PROCESS_ROLE_GUARD", tt.value)
            got, err := parseOptionalBoolEnv("TEST_PROCESS_ROLE_GUARD")
            if (err != nil) != tt.wantErr {
                t.Fatalf("parseOptionalBoolEnv() error = %v, wantErr %t", err, tt.wantErr)
            }
            if err == nil && got != tt.want {
                t.Fatalf("parseOptionalBoolEnv() = %t, want %t", got, tt.want)
            }
        })
    }
}
