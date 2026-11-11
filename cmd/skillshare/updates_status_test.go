package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
)

func TestStatusVersionOptOutSuppressesCachedSkillUpdate(t *testing.T) {
	source := t.TempDir()
	skill := filepath.Join(source, "skillshare")
	if err := os.MkdirAll(skill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nmetadata:\n  version: 1.0.0\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.AutoUpdateCheckEnv, "false")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	versioncheck.SaveRemoteSkillVersion("9.9.9")
	out := captureStdout(t, func() { checkSkillVersion(&config.Config{Source: source}) })
	if strings.Contains(out, "available") || strings.Contains(out, "9.9.9") {
		t.Fatalf("disabled status leaked update: %s", out)
	}
	if !strings.Contains(out, "1.0.0") {
		t.Fatalf("missing local version: %s", out)
	}
}
