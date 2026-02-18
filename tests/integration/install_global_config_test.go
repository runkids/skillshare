//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"

	"gopkg.in/yaml.v3"
)

func TestInstall_Global_FromConfig_SkipsExisting(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Pre-create the skill directory so it should be skipped
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
skills:
  - name: my-skill
    source: github.com/user/repo
`)

	result := sb.RunCLI("install", "--global")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skipped (already exists)")
}

func TestInstall_Global_FromConfig_EmptySkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install", "--global")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No remote skills defined")
}

func TestInstall_Global_NoSource_IncompatibleFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	tests := []struct {
		name string
		args []string
	}{
		{"name flag", []string{"install", "--global", "--name", "foo"}},
		{"into flag", []string{"install", "--global", "--into", "sub"}},
		{"track flag", []string{"install", "--global", "--track"}},
		{"skill flag", []string{"install", "--global", "--skill", "x"}},
		{"exclude flag", []string{"install", "--global", "--exclude", "x"}},
		{"all flag", []string{"install", "--global", "--all"}},
		{"yes flag", []string{"install", "--global", "--yes"}},
		{"update flag", []string{"install", "--global", "--update"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sb.RunCLI(tc.args...)
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, "require a source argument")
		})
	}
}

func TestInstall_Global_Reconcile_AfterInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a local skill directory with a recognizable name
	parentDir := t.TempDir()
	localSkill := filepath.Join(parentDir, "test-skill")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("---\nname: test-skill\n---\n# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install", "--global", localSkill)
	result.AssertSuccess(t)

	// Read back config and verify skills[] was added
	data, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var cfg struct {
		Skills []struct {
			Name   string `yaml:"name"`
			Source string `yaml:"source"`
		} `yaml:"skills"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	if len(cfg.Skills) == 0 {
		t.Fatal("expected skills[] in config after install, got none")
	}

	found := false
	for _, s := range cfg.Skills {
		if s.Name == "test-skill" {
			found = true
			if strings.TrimSpace(s.Source) == "" {
				t.Error("expected non-empty source for test-skill")
			}
		}
	}
	if !found {
		t.Errorf("expected skill 'test-skill' in config, got: %+v", cfg.Skills)
	}

	// Verify meta file was written (so reconcile can find it)
	metaPath := filepath.Join(sb.SourcePath, "test-skill", ".skillshare-meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("expected meta file at %s: %v", metaPath, err)
	}
	var meta map[string]any
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("invalid meta JSON: %v", err)
	}
	if meta["source"] == nil || strings.TrimSpace(meta["source"].(string)) == "" {
		t.Error("expected non-empty source in meta file")
	}
}
