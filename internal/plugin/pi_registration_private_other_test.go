//go:build !windows

package plugin

import (
	"os"
	"testing"
)

func assertPiRecordPrivate(t *testing.T, file string) {
	t.Helper()
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private record mode: %v", err)
	}
}
