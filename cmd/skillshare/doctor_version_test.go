package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestCheckVersionDoctor_OutdatedSkillCountsAsWarning(t *testing.T) {
	source := t.TempDir()
	skillDir := filepath.Join(source, "skillshare")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: skillshare\nmetadata:\n  version: v0.1.0\n---\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}

	remote := make(chan string, 1)
	remote <- "0.2.0"
	result := &doctorResult{}
	checkVersionDoctor(&config.Config{Source: source}, result, false, remote)

	if result.warnings != 1 {
		t.Fatalf("warnings = %d, want 1 so the summary matches --json", result.warnings)
	}
}
