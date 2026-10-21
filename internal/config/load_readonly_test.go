package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func configTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			tree[rel] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func legacyLoadFixture(t *testing.T, kind string, persist bool) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "skills")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	content := "source: " + source + "\ntargets:\n  claude:\n"
	switch kind {
	case "embedded":
		content += "    skills: {}\nskills:\n  - name: legacy\n    source: github.com/user/repo\n"
	case "flat":
		content += "    path: ~/legacy-skills\n    mode: copy\n"
	case "registry":
		content += "    skills: {}\n"
		if err := os.WriteFile(RegistryPath(root), []byte("skills:\n  - name: legacy\n    source: github.com/user/repo\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	before := configTree(t, root)
	loader := LoadReadOnly
	if persist {
		loader = Load
	}
	cfg, err := loader()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source != source || cfg.Targets["claude"].Skills == nil {
		t.Fatalf("not migrated/expanded: %+v", cfg)
	}
	if kind == "flat" {
		if cfg.Targets["claude"].Skills.Mode != "copy" || strings.HasPrefix(cfg.Targets["claude"].Skills.Path, "~") {
			t.Fatalf("flat target not migrated: %+v", cfg.Targets["claude"])
		}
	}
	if !persist {
		if after := configTree(t, root); !reflect.DeepEqual(before, after) {
			t.Fatalf("read-only load changed filesystem:\nbefore %v\nafter %v", before, after)
		}
		wantRegistryDir := source
		if kind == "registry" {
			wantRegistryDir = root
		}
		if cfg.RegistryDir != wantRegistryDir {
			t.Fatalf("RegistryDir = %q, want %q", cfg.RegistryDir, wantRegistryDir)
		}
		if kind == "registry" {
			reg, err := LoadRegistry(cfg.RegistryDir)
			if err != nil || len(reg.Skills) != 1 {
				t.Fatalf("legacy registry unavailable: %+v, %v", reg, err)
			}
		}
		return
	}
	if after := configTree(t, root); reflect.DeepEqual(before, after) {
		t.Fatal("ordinary Load did not persist migration")
	}
	if kind != "flat" {
		reg, err := LoadRegistry(source)
		if err != nil || len(reg.Skills) != 1 {
			t.Fatalf("registry not migrated: %+v, %v", reg, err)
		}
		if _, err := os.Stat(RegistryPath(root)); !os.IsNotExist(err) {
			t.Fatalf("old registry remains: %v", err)
		}
	}
}

func TestLoadReadOnly_EmbeddedSkillsNoRegistry(t *testing.T) { legacyLoadFixture(t, "embedded", false) }
func TestLoadReadOnly_FlatTargets(t *testing.T)              { legacyLoadFixture(t, "flat", false) }
func TestLoadReadOnly_OldRegistryLocation(t *testing.T)      { legacyLoadFixture(t, "registry", false) }
func TestLoad_MigratesEmbeddedSkills(t *testing.T)           { legacyLoadFixture(t, "embedded", true) }
func TestLoad_MigratesFlatTargets(t *testing.T)              { legacyLoadFixture(t, "flat", true) }
func TestLoad_MigratesOldRegistryLocation(t *testing.T)      { legacyLoadFixture(t, "registry", true) }

func TestLoadReadOnly_WindowsLegacyConfigPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "current"))
	t.Setenv("SKILLSHARE_CONFIG", "")
	legacy := filepath.Join(root, ".config", "skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before := configTree(t, root)
	if got := readOnlyConfigPath("windows"); got != legacy {
		t.Fatalf("path = %q, want legacy %q", got, legacy)
	}
	cfg, err := loadConfig(readOnlyConfigPath("windows"), false)
	if err != nil || cfg == nil {
		t.Fatalf("legacy config not readable: %v", err)
	}
	if after := configTree(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("legacy Windows config relocated")
	}
	if got := readOnlyConfigPath("linux"); got != ConfigPath() {
		t.Fatalf("non-Windows selected legacy path: %q", got)
	}
	current := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(current), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := readOnlyConfigPath("windows"); got != current {
		t.Fatalf("existing config ignored: %q", got)
	}
	override := filepath.Join(root, "override.yaml")
	t.Setenv("SKILLSHARE_CONFIG", override)
	if got := readOnlyConfigPath("windows"); got != override {
		t.Fatalf("explicit config override ignored: %q", got)
	}
}

func TestLoadReadOnly_PrepareSaveRetainsEmbeddedSkills(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	original := "source: " + root + "/skills\ntargets: {}\nskills:\n  - name: legacy\n    source: github.com/user/repo\n    custom: retained\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Mode = "copy"
	c, err := PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c.Bytes), "name: legacy") || !strings.Contains(string(c.Bytes), "custom: retained") {
		t.Fatalf("embedded skills lost: %s", c.Bytes)
	}
	if err := c.Write(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(RegistryPath(root)); !os.IsNotExist(err) {
		t.Fatalf("unexpected registry migration: %v", err)
	}
}
