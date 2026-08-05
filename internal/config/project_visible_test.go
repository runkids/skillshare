package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeVisibleProject creates a project marked by a visible skillshare/ directory.
func writeVisibleProject(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `targets:
  - claude
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProjectConfigPath_VisibleDirectory(t *testing.T) {
	root := t.TempDir()
	dir := writeVisibleProject(t, root)

	if got, want := ProjectConfigPath(root), filepath.Join(dir, "config.yaml"); got != want {
		t.Errorf("ProjectConfigPath() = %q, want %q", got, want)
	}
}

func TestProjectConfigPath_HiddenWinsOverVisible(t *testing.T) {
	root := t.TempDir()
	writeVisibleProject(t, root)
	hidden := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(hidden, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "config.yaml"), []byte("targets: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if got, want := ProjectConfigPath(root), filepath.Join(hidden, "config.yaml"); got != want {
		t.Errorf("ProjectConfigPath() = %q, want %q", got, want)
	}
}

func TestLoadProject_VisibleDirectory(t *testing.T) {
	root := t.TempDir()
	writeVisibleProject(t, root)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Name != "claude" {
		t.Fatalf("targets = %+v, want one claude target", cfg.Targets)
	}
}

func TestEffectiveSources_FollowVisibleDirectory(t *testing.T) {
	root := t.TempDir()
	dir := writeVisibleProject(t, root)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"skills", cfg.EffectiveSkillsSource(root), filepath.Join(dir, "skills")},
		{"agents", cfg.EffectiveAgentsSource(root), filepath.Join(dir, "agents")},
		{"extras", cfg.EffectiveExtrasSource(root), filepath.Join(dir, "extras")},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s source = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// sources: must still win over the project directory default, and still resolve
// from the project root rather than from the marker.
func TestEffectiveSources_CustomSourcesStillWin(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `sources:
  skills: ./docs/skills
targets:
  - claude
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if got, want := cfg.EffectiveSkillsSource(root), filepath.Join(root, "docs", "skills"); got != want {
		t.Errorf("skills source = %q, want %q", got, want)
	}
}

func TestSaveIn_WritesToGivenDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skillshare")

	cfg := &ProjectConfig{Targets: []ProjectTargetEntry{{Name: "claude"}}}
	if err := cfg.SaveIn(dir); err != nil {
		t.Fatalf("SaveIn failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config not written to %s: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".skillshare")); !os.IsNotExist(err) {
		t.Error("SaveIn created a hidden directory")
	}

	// The saved project must now resolve to the visible directory.
	if got, want := ProjectConfigPath(root), filepath.Join(dir, "config.yaml"); got != want {
		t.Errorf("ProjectConfigPath() = %q, want %q", got, want)
	}
}

func TestProjectGitignoreTarget_VisibleDirectory(t *testing.T) {
	root := t.TempDir()
	dir := writeVisibleProject(t, root)

	gitignoreDir, prefix := ProjectGitignoreTarget(root, filepath.Join(dir, "skills"))
	if gitignoreDir != dir {
		t.Errorf("gitignoreDir = %q, want %q", gitignoreDir, dir)
	}
	if prefix != "skills" {
		t.Errorf("entryPrefix = %q, want %q", prefix, "skills")
	}
}
