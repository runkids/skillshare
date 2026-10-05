package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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
		{"schemas/mcp.schema.json", "Skillshare MCP Source"},
		{"schemas/hooks.schema.json", "Skillshare Hooks"},
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

func TestSchema_ExtrasAndAgentsAllowExtension(t *testing.T) {
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
		if !owners["agents"] {
			t.Errorf("%s: agents block missing 'extension' property", file)
		}
	}
}

func TestSchema_ExtrasAllowSingleFile(t *testing.T) {
	root := findRepoRoot(t)
	for _, file := range []string{"schemas/config.schema.json", "schemas/project-config.schema.json"} {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var schema struct {
			Defs map[string]struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"$defs"`
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: invalid JSON: %v", file, err)
		}
		if _, ok := schema.Defs["extraConfig"].Properties["file"]; !ok {
			t.Errorf("%s: extraConfig missing 'file' property", file)
		}
		target := schema.Defs["extraTargetConfig"].Properties
		if _, ok := target["as"]; !ok {
			t.Errorf("%s: extraTargetConfig missing 'as' property", file)
		}
		if !slices.Contains(target["mode"].Enum, "import") {
			t.Errorf("%s: extraTargetConfig mode enum missing 'import': %v", file, target["mode"].Enum)
		}
	}
}

func TestSchema_TargetsAllowInstructions(t *testing.T) {
	root := findRepoRoot(t)
	cases := map[string]func(defs map[string]any) any{
		"schemas/config.schema.json": func(defs map[string]any) any {
			return defs["targetConfig"]
		},
		"schemas/project-config.schema.json": func(defs map[string]any) any {
			return defs["projectTargetEntry"].(map[string]any)["oneOf"].([]any)[1]
		},
	}
	for file, target := range cases {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: invalid JSON: %v", file, err)
		}
		props := target(schema["$defs"].(map[string]any)).(map[string]any)["properties"].(map[string]any)
		in, ok := props["instructions"].(map[string]any)
		if !ok {
			t.Fatalf("%s: target missing 'instructions' property", file)
		}
		inProps := in["properties"].(map[string]any)
		for _, key := range []string{"path", "import"} {
			if _, ok := inProps[key]; !ok {
				t.Errorf("%s: instructions missing %q", file, key)
			}
		}
	}
}

func TestSchema_TargetSkillsAllowEnabled(t *testing.T) {
	root := findRepoRoot(t)
	cases := map[string]func(defs map[string]any) any{
		"schemas/config.schema.json": func(defs map[string]any) any {
			return defs["targetConfig"]
		},
		"schemas/project-config.schema.json": func(defs map[string]any) any {
			return defs["projectTargetEntry"].(map[string]any)["oneOf"].([]any)[1]
		},
	}
	for file, target := range cases {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: invalid JSON: %v", file, err)
		}
		props := target(schema["$defs"].(map[string]any)).(map[string]any)["properties"].(map[string]any)
		skills := props["skills"].(map[string]any)["properties"].(map[string]any)
		if _, ok := skills["enabled"]; !ok {
			t.Errorf("%s: skills missing 'enabled'", file)
		}
		agents := props["agents"].(map[string]any)["properties"].(map[string]any)
		if _, ok := agents["enabled"]; ok {
			t.Errorf("%s: agents must not accept 'enabled'", file)
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

func TestSchema_TargetsAllowFiles(t *testing.T) {
	root := findRepoRoot(t)
	cases := map[string]func(defs map[string]any) any{
		"schemas/config.schema.json": func(defs map[string]any) any {
			return defs["targetConfig"]
		},
		"schemas/project-config.schema.json": func(defs map[string]any) any {
			return defs["projectTargetEntry"].(map[string]any)["oneOf"].([]any)[1]
		},
	}
	for file, target := range cases {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: invalid JSON: %v", file, err)
		}
		props := target(schema["$defs"].(map[string]any)).(map[string]any)["properties"].(map[string]any)
		if files, ok := props["files"].(map[string]any); !ok || files["type"] != "array" {
			t.Errorf("%s: target missing 'files' array property", file)
		}
	}
}

func TestSchema_HooksAllowAccountBindings(t *testing.T) {
	data, err := os.ReadFile("../../schemas/hooks.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	defs := schema["$defs"].(map[string]any)
	gb, ok := defs["globalBindings"].(map[string]any)
	if !ok {
		t.Fatal("globalBindings missing")
	}
	pattern := `^(?!(claude|codex|gemini|qwen|copilot|cursor|droid|factory|antigravity|antigravity-cli|agy|pi|omp|amp|opencode|git)$)[A-Za-z0-9][A-Za-z0-9._-]*$`
	patterns := gb["patternProperties"].(map[string]any)
	if len(patterns) != 1 || patterns[pattern] == nil {
		t.Fatalf("%v", patterns)
	}
	refs := patterns[pattern].(map[string]any)["anyOf"].([]any)
	if len(refs) != 2 || refs[0].(map[string]any)["$ref"] != "#/$defs/commandBinding" || refs[1].(map[string]any)["$ref"] != "#/$defs/codeBinding" {
		t.Fatalf("%v", refs)
	}
	global := defs["globalConfiguration"].(map[string]any)["properties"].(map[string]any)
	if global["entries"].(map[string]any)["$ref"] != "#/$defs/globalEntries" {
		t.Fatal("global entries strict")
	}
	project := global["projects"].(map[string]any)["additionalProperties"].(map[string]any)["properties"].(map[string]any)
	if project["entries"].(map[string]any)["$ref"] != "#/$defs/entries" || schema["$ref"] != "#/$defs/globalEntry" {
		t.Fatal("incorrect project or root scope")
	}
}
