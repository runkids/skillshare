package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPrepareSave_MatchesSave(t *testing.T) {
	for _, fold := range []bool{false, true} {
		t.Run(fmt.Sprint(fold), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("USERPROFILE", dir)
			path := filepath.Join(dir, "config.yaml")
			t.Setenv("SKILLSHARE_CONFIG", path)
			original := []byte("# my config\nsource: ~/skills # shared source\nmode: &mode copy\ntargets:\n  claude:\n    skills:\n      mode: *mode\n")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.PreserveTildeOnSave = fold
			candidate, err := PrepareSave(path, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
				t.Fatal("PrepareSave wrote config")
			}
			// The public save and separately prepared candidate must serialize identically.
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(candidate.Bytes, saved) {
				t.Fatalf("candidate differs from Save:\n%s\nSave:\n%s", candidate.Bytes, saved)
			}
			for _, want := range []string{"# my config", "# shared source", "&mode", "*mode", string(schemaComment)} {
				if !strings.Contains(string(candidate.Bytes), want) {
					t.Errorf("candidate lost %q:\n%s", want, candidate.Bytes)
				}
			}
			if candidate.Expanded == nil || candidate.Expanded.Source != filepath.Join(dir, "skills") {
				t.Fatalf("candidate not expanded: %+v", candidate.Expanded)
			}
		})
	}
}

func TestPrepareSave_OmitsSyntheticTargets(t *testing.T) {
	cfg, _, err := loadWithProjects(t, "  $ROOT/app:\n    targets: [claude]\n    skills: {}\n", "app")
	if err != nil {
		t.Fatal(err)
	}
	c, err := PrepareSave(ConfigPath(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var stored Config
	if err := yaml.Unmarshal(c.Bytes, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Targets) != 0 || len(stored.Projects) != 1 {
		t.Fatalf("stored targets/projects = %v / %v", stored.Targets, stored.Projects)
	}
	if c.Expanded == nil || len(c.Expanded.Targets) != 1 {
		t.Fatalf("expanded = %+v", c.Expanded)
	}
}

func TestPrepareSave_ExpandedReflectsRename(t *testing.T) {
	cfg, root, err := loadWithProjects(t, "  $ROOT/app:\n    targets: [claude]\n    skills: {}\n", "app")
	if err != nil {
		t.Fatal(err)
	}
	// Match the declared fixture spelling, which intentionally uses a slash.
	app := root + "/app"
	project, ok := cfg.Projects[app]
	if !ok {
		t.Fatal("fixture project is missing")
	}
	project.Name = "renamed"
	cfg.Projects[app] = project
	c, err := PrepareSave(ConfigPath(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Expanded == nil {
		t.Fatal("missing expanded view")
	}
	if _, ok := c.Expanded.Targets["renamed@claude"]; !ok {
		t.Fatalf("new target missing: %v", c.Expanded.Targets)
	}
	if _, ok := c.Expanded.Targets["app@claude"]; ok {
		t.Fatal("stale synthetic target")
	}
}

func TestPrepareSave_CallerUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	cfg := &Config{Source: filepath.Join(dir, "skills"), PreserveTildeOnSave: true,
		Targets: map[string]TargetConfig{"claude": {Skills: &ResourceTargetConfig{Path: filepath.Join(dir, ".claude", "skills"), Include: []string{"keep"}}}},
		Extras:  []ExtraConfig{{Name: "rules", Source: filepath.Join(dir, "rules"), Targets: []ExtraTargetConfig{{Path: filepath.Join(dir, ".rules")}}}},
	}
	before, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot Config
	if err := yaml.Unmarshal(before, &snapshot); err != nil {
		t.Fatal(err)
	}
	c, err := PrepareSave(filepath.Join(dir, "missing", "config.yaml"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Expanded == nil {
		t.Fatal("missing expanded view")
	}
	if !reflect.DeepEqual(cfg, &snapshot) {
		t.Fatalf("caller changed: %+v", cfg)
	}
	c.Expanded.Targets["claude"].Skills.Include[0] = "changed"
	if cfg.Targets["claude"].Skills.Include[0] != "keep" {
		t.Fatal("expanded view aliases caller")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Fatalf("PrepareSave created directory: %v", err)
	}
}

func TestPrepareSave_ValidationErrorDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("targets: {}\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Projects: map[string]ManagedProject{dir: {Name: "bad@name", Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}}}
	c, err := PrepareSave(path, cfg)
	if err == nil || c == nil || c.Err != err {
		t.Fatalf("candidate/error contract: %+v, %v", c, err)
	}
	if writeErr := c.Write(); writeErr != err {
		t.Fatalf("Write error = %v, want %v", writeErr, err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("invalid candidate wrote config")
	}
}

func TestPrepareSave_WriteUsesPreparedBytesAndKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shared.yaml")
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(target, []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Mode: "copy"}
	c, err := PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A subsequent caller edit or file comment must not trigger serialization again.
	cfg.Mode = "merge"
	if err := os.WriteFile(target, []byte("# changed since preparation\ntargets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := c.Write(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, c.Bytes) || !strings.Contains(string(got), "mode: copy") {
		t.Fatalf("wrong bytes written:\n%s", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}

func TestPrepareSave_ValidatesEffectiveAnchoredProjects(t *testing.T) {
	cfg, root, err := loadWithProjects(t, "  $ROOT/app: &shared\n    targets: [claude]\n    skills: {}\n  $ROOT/other: *shared\n", "app", "other")
	if err != nil {
		t.Fatal(err)
	}
	// Match the exact declared keys rather than normalizing their separators.
	app, other := root+"/app", root+"/other"
	project, ok := cfg.Projects[app]
	if !ok {
		t.Fatal("fixture project is missing")
	}
	project.Name = "renamed"
	cfg.Projects[app] = project
	original, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	c, err := PrepareSave(ConfigPath(), cfg)
	if err == nil || !strings.Contains(err.Error(), "both named") || c.Err != err {
		t.Fatalf("inherited duplicate name accepted: %v\n%s", err, c.Bytes)
	}
	if !strings.Contains(string(c.Bytes), "*shared") {
		t.Fatalf("project alias lost:\n%s", c.Bytes)
	}
	if cfg.Projects[other].Name != "" {
		t.Fatal("candidate expansion changed caller's aliased project")
	}
	if got, _ := os.ReadFile(ConfigPath()); !bytes.Equal(got, original) {
		t.Fatal("invalid effective candidate wrote config")
	}
}

func TestPrepareSave_ExpandedRegistryUsesCanonicalSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "skills")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(RegistryPath(dir), []byte("skills: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := PrepareSave(path, &Config{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if c.Expanded.RegistryDir != source {
		t.Fatalf("candidate retained legacy registry location: %s", c.Expanded.RegistryDir)
	}
	if _, err := os.Stat(RegistryPath(source)); !os.IsNotExist(err) {
		t.Fatalf("preparation relocated registry: %v", err)
	}
}

func TestPrepareSave_WriteFailureKeepsConfigAndCaller(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	original := []byte("mode: merge\ntargets: {}\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Mode: "copy"}
	c, err := PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A filesystem change between preparation and write makes the parent unusable.
	moved := filepath.Join(root, "moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	marker := []byte("unavailable config directory")
	if err := os.WriteFile(dir, marker, 0644); err != nil {
		t.Fatal(err)
	}
	if err := c.Write(); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	if got, _ := os.ReadFile(filepath.Join(moved, "config.yaml")); !bytes.Equal(got, original) {
		t.Fatalf("config changed on failed write: %s", got)
	}
	if got, _ := os.ReadFile(dir); !bytes.Equal(got, marker) {
		t.Fatalf("blocking file changed: %s", got)
	}
	if !reflect.DeepEqual(cfg, &Config{Mode: "copy"}) {
		t.Fatalf("caller changed on failed write: %+v", cfg)
	}
}
