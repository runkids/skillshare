package main

import (
	"fmt"
	"testing"

	"skillshare/internal/audit"
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

		// Sentinel-based detection
		{fmt.Errorf("security audit failed: scan error: %w", audit.ErrBlocked), true},
		{fmt.Errorf("post-update audit failed: scan error — rolled back: %w", audit.ErrBlocked), true},
		{fmt.Errorf("rollback failed: permission denied: %w", audit.ErrBlocked), true},
		{audit.ErrBlocked, true},

		// Wrapped sentinel
		{fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", audit.ErrBlocked)), true},

		// Non-sentinel errors with similar text should NOT match
		{fmt.Errorf("security audit something"), false},
		{fmt.Errorf("was rolled back successfully"), false},
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
