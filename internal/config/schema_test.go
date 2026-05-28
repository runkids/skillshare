package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSave_IncludesSchemaComment(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	cfg := &Config{
		Source:  "/tmp/skills",
		Targets: map[string]TargetConfig{},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	want := "# yaml-language-server: $schema=" + GlobalSchemaURL
	if firstLine != want {
		t.Errorf("first line = %q, want %q", firstLine, want)
	}
}

func TestProjectSave_IncludesSchemaComment(t *testing.T) {
	root := t.TempDir()

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}

	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	path := ProjectConfigPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read project config: %v", err)
	}

	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	want := "# yaml-language-server: $schema=" + ProjectSchemaURL
	if firstLine != want {
		t.Errorf("first line = %q, want %q", firstLine, want)
	}
}

func TestLoad_WithSchemaComment(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	raw := "# yaml-language-server: $schema=" + GlobalSchemaURL + "\nsource: /tmp/skills\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Source != "/tmp/skills" {
		t.Errorf("Source = %q, want /tmp/skills", cfg.Source)
	}
}

func TestLoad_WithTargetNaming(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	raw := "# yaml-language-server: $schema=" + GlobalSchemaURL + "\nsource: /tmp/skills\ntarget_naming: standard\ntargets:\n  claude:\n    skills:\n      target_naming: flat\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TargetNaming != "standard" {
		t.Fatalf("TargetNaming = %q, want standard", cfg.TargetNaming)
	}
	claude := cfg.Targets["claude"]
	if got := claude.SkillsConfig().TargetNaming; got != "flat" {
		t.Fatalf("target target_naming = %q, want flat", got)
	}
}

func TestLoadProject_WithSchemaComment(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	raw := "# yaml-language-server: $schema=" + ProjectSchemaURL + "\ntargets:\n  - claude\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Name != "claude" {
		t.Errorf("unexpected targets: %+v", cfg.Targets)
	}
}

func TestLoadProject_WithTargetNaming(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	raw := "# yaml-language-server: $schema=" + ProjectSchemaURL + "\ntarget_naming: standard\ntargets:\n  - name: claude\n    skills:\n      target_naming: flat\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if cfg.TargetNaming != "standard" {
		t.Fatalf("TargetNaming = %q, want standard", cfg.TargetNaming)
	}
	if got := cfg.Targets[0].SkillsConfig().TargetNaming; got != "flat" {
		t.Fatalf("target target_naming = %q, want flat", got)
	}
}

func TestSchemaFiles_ValidJSON(t *testing.T) {
	// Find schema files relative to this test file's package.
	// Schema files are at the repo root: schemas/*.json
	root := findRepoRoot(t)

	tests := []struct {
		file      string
		wantTitle string
	}{
		{"schemas/config.schema.json", "Skillshare Global Configuration"},
		{"schemas/project-config.schema.json", "Skillshare Project Configuration"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join(root, tt.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read schema file: %v", err)
			}

			var schema map[string]any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			if title, ok := schema["title"].(string); !ok || title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if _, ok := schema["$defs"]; !ok {
				t.Error("schema should have $defs")
			}
		})
	}
}

func TestSchema_ExtraTargetsAllowExtension(t *testing.T) {
	root := findRepoRoot(t)
	for _, file := range []string{"schemas/config.schema.json", "schemas/project-config.schema.json"} {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: invalid JSON: %v", file, err)
		}
		owners := map[string]bool{}
		collectExtensionOwners(schema, owners)
		if !owners["extraTargetConfig"] {
			t.Errorf("%s: extraTargetConfig missing 'extension' property", file)
		}
		if owners["agents"] {
			t.Errorf("%s: agents block must not expose 'extension' (agent extensions are unsupported)", file)
		}
	}
}

// collectExtensionOwners records the key of every object whose "properties"
// includes an "extension" field, so schema tests can assert which blocks accept
// an extension transform.
func collectExtensionOwners(node any, owners map[string]bool) {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			if vm, ok := v.(map[string]any); ok {
				if props, ok := vm["properties"].(map[string]any); ok {
					if _, has := props["extension"]; has {
						owners[k] = true
					}
				}
			}
			collectExtensionOwners(v, owners)
		}
	case []any:
		for _, item := range n {
			collectExtensionOwners(item, owners)
		}
	}
}

// findRepoRoot walks up from the current working directory to find go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}
