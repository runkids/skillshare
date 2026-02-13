package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectTargetEntryMarshal_WithFiltersUsesObjectForm(t *testing.T) {
	entry := ProjectTargetEntry{
		Name:    "codex",
		Include: []string{"codex-*"},
		Exclude: []string{"codex-test"},
	}

	v, err := entry.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML returned error: %v", err)
	}

	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected object form, got %T", v)
	}
	if obj["name"] != "codex" {
		t.Fatalf("name = %v, want codex", obj["name"])
	}
	if got, ok := obj["include"].([]string); !ok || !reflect.DeepEqual(got, []string{"codex-*"}) {
		t.Fatalf("include = %#v, want []string{\"codex-*\"}", obj["include"])
	}
	if got, ok := obj["exclude"].([]string); !ok || !reflect.DeepEqual(got, []string{"codex-test"}) {
		t.Fatalf("exclude = %#v, want []string{\"codex-test\"}", obj["exclude"])
	}
}

func TestLoadProjectAndResolveTargets_PreservesFilters(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}

	raw := "targets:\n" +
		"  - name: codex\n" +
		"    path: ./.agents/skills\n" +
		"    include: [codex-*]\n" +
		"    exclude: [codex-test]\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject returned error: %v", err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(cfg.Targets))
	}
	if !reflect.DeepEqual(cfg.Targets[0].Include, []string{"codex-*"}) {
		t.Fatalf("target include = %v", cfg.Targets[0].Include)
	}
	if !reflect.DeepEqual(cfg.Targets[0].Exclude, []string{"codex-test"}) {
		t.Fatalf("target exclude = %v", cfg.Targets[0].Exclude)
	}

	resolved, err := ResolveProjectTargets(root, cfg)
	if err != nil {
		t.Fatalf("ResolveProjectTargets returned error: %v", err)
	}
	target, ok := resolved["codex"]
	if !ok {
		t.Fatal("expected resolved codex target")
	}
	if !reflect.DeepEqual(target.Include, []string{"codex-*"}) {
		t.Fatalf("resolved include = %v", target.Include)
	}
	if !reflect.DeepEqual(target.Exclude, []string{"codex-test"}) {
		t.Fatalf("resolved exclude = %v", target.Exclude)
	}
}
