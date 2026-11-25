package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
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

func TestCheckVersionDoctor_DisabledSkillCountsAsWarning(t *testing.T) {
	for _, tt := range []struct {
		isProject bool
		want      string
	}{
		{false, "skillshare enable skillshare -g && skillshare sync -g"},
		{true, "skillshare enable skillshare -p && skillshare sync -p"},
	} {
		result := checkDisabledSkillDoctor(t, tt.isProject)
		last := result.checks[len(result.checks)-1]
		if result.warnings != 1 || last.Name != "skill_version" || last.Status != checkWarning {
			t.Fatalf("warnings = %d, last check = %+v; want a skill_version warning for a disabled skill", result.warnings, last)
		}
		if len(result.next) == 0 || result.next[0] != tt.want {
			t.Errorf("project %v: next = %v, want %q first", tt.isProject, result.next, tt.want)
		}
	}
}

// checkDisabledSkillDoctor runs the version check on a source whose
// .skillignore disables the built-in skill.
func checkDisabledSkillDoctor(t *testing.T, isProject bool) *doctorResult {
	t.Helper()
	source := t.TempDir()
	skillDir := filepath.Join(source, "skillshare")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: skillshare\nmetadata:\n  version: v0.2.0\n---\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillignore"), []byte("skillshare\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	remote := make(chan string, 1)
	remote <- "0.2.0"
	result := &doctorResult{}
	captureStdout(t, func() { checkVersionDoctor(&config.Config{Source: source}, result, isProject, remote) })
	return result
}

func TestPrintUpdateAvailable_NixAdvice(t *testing.T) {
	tests := []struct {
		name     string
		method   versioncheck.InstallMethod
		wantRow  string
		wantNext []string
	}{
		{name: "nix profile", method: versioncheck.InstallNixProfile, wantNext: []string{"nix profile upgrade skillshare"}},
		{name: "nix configuration", method: versioncheck.InstallNix, wantRow: versioncheck.NixConfigUpgradeHint},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := &versioncheck.CheckResult{CurrentVersion: "1.0.0", LatestVersion: "1.1.0", UpdateAvailable: true, InstallMethod: tt.method}
			result := &doctorResult{}
			out := captureStdout(t, func() { printUpdateAvailable(update, result) })

			if !strings.Contains(out, tt.wantRow) {
				t.Errorf("row %q lacks %q", out, tt.wantRow)
			}
			var next []string
			for i := 0; i < len(result.next); i += 2 {
				next = append(next, result.next[i])
			}
			if !slices.Equal(next, tt.wantNext) {
				t.Errorf("next = %v, want %v", next, tt.wantNext)
			}
		})
	}
}
