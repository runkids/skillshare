//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestCLISearch_OfflineStartup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// A legacy backup must not be moved by startup maintenance, even when the
	// destination does not yet exist. Config must not be needed for discovery.
	legacy := filepath.Join(filepath.Dir(sb.ConfigPath), "backups", "sentinel")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	for _, configContents := range []string{"", "invalid: [yaml"} {
		if configContents != "" {
			if err := os.WriteFile(sb.ConfigPath, []byte(configContents), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, args := range [][]string{
			{"cli", "search", "sync", "--json"},
			{"cli", "search", "--json"},
			{"cli", "--help"},
		} {
			result := sb.RunCLI(args...)
			if len(args) == 3 && args[2] == "--json" {
				if result.ExitCode != 1 {
					t.Fatalf("expected usage exit 1: %+v", result)
				}
			} else {
				result.AssertSuccess(t)
			}
			if result.Stderr != "" {
				t.Fatalf("unexpected stderr: %s", result.Stderr)
			}
			if args[1] == "search" && !json.Valid([]byte(result.Stdout)) {
				t.Fatalf("not pure JSON: %s", result.Stdout)
			}
			if contents, err := os.ReadFile(legacy); err != nil || string(contents) != "keep" {
				t.Fatalf("legacy backup changed: %q / %v", contents, err)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatalf("migration destination created: %v", err)
			}
		}
	}
}
