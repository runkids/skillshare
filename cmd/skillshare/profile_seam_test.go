package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestProfiles_UnavailableEmptySourceCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	if err := os.WriteFile(path, []byte("targets: {}\n"), 0600); err != nil { t.Fatal(err) }
	statePath, _, err := config.ProfilePaths(path)
	if err != nil { t.Fatal(err) }
	if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "missing"}); err != nil { t.Fatal(err) }
	if err := cmdDiffGlobal("", kindAll, diffRenderOpts{jsonOutput: true}, time.Now()); !errors.Is(err, profile.ErrProfileUnavailable) {
		t.Fatalf("diff accepted unavailable profile: %v", err)
	}
	if err := runAnalyze(&analyzeOptions{json: true}); !errors.Is(err, profile.ErrProfileUnavailable) {
		t.Fatalf("analyze accepted unavailable profile: %v", err)
	}
}
