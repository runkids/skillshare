package main

import (
	"fmt"
	"testing"
)

func TestIsSecurityError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("connection timeout"), false},
		{fmt.Errorf("skill not found"), false},
		{fmt.Errorf("has uncommitted changes"), false},

		// auditGateAfterPull errors
		{fmt.Errorf("security audit failed: scan error (use --skip-audit to bypass)"), true},
		{fmt.Errorf("security audit found HIGH/CRITICAL findings — rolled back (use --skip-audit to bypass)"), true},
		{fmt.Errorf("update rejected by user after security audit"), true},
		{fmt.Errorf("rollback failed: permission denied"), true},

		// install.Install post-update audit errors
		{fmt.Errorf("post-update audit failed: scan error (use --skip-audit to bypass)"), true},
		{fmt.Errorf("post-update audit found HIGH/CRITICAL findings — rolled back (use --skip-audit to bypass)"), true},

		// install.Install install-time audit errors
		{fmt.Errorf("security audit failed — findings at/above HIGH detected"), true},

		// Partial keyword matches should still work
		{fmt.Errorf("something security audit something"), true},
		{fmt.Errorf("was rolled back successfully"), true},
	}

	for _, tt := range tests {
		name := "nil"
		if tt.err != nil {
			name = tt.err.Error()
			if len(name) > 60 {
				name = name[:60]
			}
		}
		t.Run(name, func(t *testing.T) {
			got := isSecurityError(tt.err)
			if got != tt.want {
				t.Errorf("isSecurityError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
