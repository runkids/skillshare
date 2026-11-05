package config

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalProjectNameConflictWithMissingShared(t *testing.T) {
	t.Run("both missing", func(t *testing.T) {
		base := t.TempDir()
		local := filepath.Join(base, "local", "app")
		shared := filepath.Join(base, "shared", "app")
		project := ManagedProject{Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}
		cfg := &Config{Projects: map[string]ManagedProject{local: project, shared: project}, projectOverlay: &projectOverlay{origins: map[string]string{local: "local", shared: "shared"}}}
		if err := cfg.expandProjects(); err != nil {
			t.Fatalf("missing projects block load: %v", err)
		}
		if err := cfg.ValidateProjects(cfg.Projects); err != nil {
			t.Fatalf("missing projects block validation: %v", err)
		}
		if len(cfg.Targets) != 0 {
			t.Fatalf("missing projects produced targets: %v", cfg.Targets)
		}
	})
	t.Run("missing local", func(t *testing.T) {
		base := t.TempDir()
		local := filepath.Join(base, "local", "app")
		shared := filepath.Join(base, "shared", "app")
		if err := os.MkdirAll(shared, 0755); err != nil {
			t.Fatal(err)
		}
		project := ManagedProject{Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}
		cfg := &Config{Projects: map[string]ManagedProject{local: project, shared: project}, projectOverlay: &projectOverlay{origins: map[string]string{local: "local", shared: "shared"}}}
		if err := cfg.expandProjects(); err != nil {
			t.Fatalf("missing local project blocks load: %v", err)
		}
		if target, ok := cfg.Targets["app@claude"]; !ok || target.ProjectRoot() != shared {
			t.Fatalf("shared target missing: %v", cfg.Targets)
		}
	})
	base := t.TempDir()
	local := filepath.Join(base, "local", "app")
	shared := filepath.Join(base, "shared", "app")
	if err := os.MkdirAll(local, 0755); err != nil {
		t.Fatal(err)
	}
	project := ManagedProject{Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}
	cfg := &Config{Projects: map[string]ManagedProject{local: project, shared: project}, projectOverlay: &projectOverlay{origins: map[string]string{local: "local", shared: "shared"}}}
	if err := cfg.expandProjects(); err != nil {
		t.Fatalf("missing shared project blocks load: %v", err)
	}
	if err := cfg.ValidateProjects(cfg.Projects); err != nil {
		t.Fatalf("missing shared project blocks validation: %v", err)
	}
	if _, ok := cfg.Targets["app@claude"]; !ok {
		t.Fatal("local target was not expanded")
	}
	if err := os.MkdirAll(shared, 0755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.expandProjects(); err == nil {
		t.Fatal("two existing projects with the same name were accepted")
	}
}

func TestLocalProjectLoadsWithReplicatedMissingSharedName(t *testing.T) {
	cfg, base, err := loadWithProjects(t, "  $ROOT/remote/app: {targets: [claude], skills: {}}\n", "local/app")
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(base, "local", "app")
	entries := maps.Clone(cfg.Projects)
	entries[local] = ManagedProject{Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}
	candidate, err := PrepareLocalProjects(ConfigPath(), cfg, entries, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Write(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("replicated missing shared project blocked load: %v", err)
	}
	if target, ok := loaded.Targets["app@claude"]; !ok || target.ProjectRoot() != local {
		t.Fatalf("local target missing: %+v", loaded.Targets)
	}
}

func TestLocalProjectsOverlayAndSaveIsolation(t *testing.T) {
	cfg, root, err := loadWithProjects(t, "  $ROOT/app:\n    targets: [claude]\n    skills: {}\n", "app")
	if err != nil {
		t.Fatal(err)
	}
	path := ConfigPath()
	original, _ := os.ReadFile(path)
	entries := map[string]ManagedProject{filepath.Join(root, "app"): {Name: "local", Targets: []string{"codex"}, Skills: &ResourceTargetConfig{}}}
	c, err := PrepareLocalProjects(path, cfg, entries, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("local save changed shared config")
	}
	cfg, err = LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Projects[filepath.Join(root, "app")].Name != "local" {
		t.Fatal(cfg.Projects)
	}
	if origin, _ := cfg.ProjectOrigin(filepath.Join(root, "app")); origin != "local" {
		t.Fatal(origin)
	}
	for _, prepare := range []bool{true, false} {
		if prepare {
			c, err := PrepareSave(path, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(c.Bytes, []byte("name: local")) {
				t.Fatal("PrepareSave leaked local entry")
			}
		} else if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
	}
	shared, _ := os.ReadFile(path)
	if bytes.Contains(shared, []byte("name: local")) {
		t.Fatal("Save leaked local entry")
	}
}

func TestLocalProjectsRejectStaleSnapshots(t *testing.T) {
	for _, changed := range []string{"local", "shared"} {
		t.Run(changed, func(t *testing.T) {
			cfg, root, err := loadWithProjects(t, "", "app")
			if err != nil {
				t.Fatal(err)
			}
			entries := map[string]ManagedProject{filepath.Join(root, "app"): {Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}}
			first, err := PrepareLocalProjects(ConfigPath(), cfg, entries, "")
			if err != nil {
				t.Fatal(err)
			}
			second, err := PrepareLocalProjects(ConfigPath(), cfg, entries, "")
			if err != nil {
				t.Fatal(err)
			}
			if changed == "local" {
				if err := first.Write(); err != nil {
					t.Fatal(err)
				}
			} else {
				f, err := os.OpenFile(ConfigPath(), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.WriteString("# external change\n")
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := second.Check(); err == nil {
				t.Fatal("stale snapshot accepted")
			}
			if err := second.Write(); err == nil {
				t.Fatal("stale snapshot written")
			}
		})
	}
}

func TestLocalProjectsInvalidStateFailsClosed(t *testing.T) {
	cfg, _, err := loadWithProjects(t, "", "app")
	if err != nil {
		t.Fatal(err)
	}
	_ = cfg
	path, err := LocalProjectsPath(ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", "version: 2\n", "projects: {}\n", "version: 1\nunknown: true\n", "version: 1\n---\nversion: 1\n", "version: 1\nexcluded: [relative]\n"} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadReadOnly(); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestLocalProjectsWindowsStorageAndMigrationIdentity(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	if got := projectStateDir("windows"); got != filepath.Join(root, "local", "skillshare", "state") {
		t.Fatal(got)
	}
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("SKILLSHARE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "roaming"))
	legacy := filepath.Join(root, ".config", "skillshare", "config.yaml")
	old, err := projectConfigIdentity(legacy, "windows")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := projectConfigIdentity(ConfigPath(), "windows")
	if err != nil {
		t.Fatal(err)
	}
	if old != canonical {
		t.Fatalf("legacy %s != canonical %s", old, canonical)
	}
}

func TestLocalProjectsWindowsMigrationIdentityWithResolvedHome(t *testing.T) {
	root := t.TempDir()
	realHome := filepath.Join(root, "real-home")
	if err := os.Mkdir(realHome, 0700); err != nil {
		t.Fatal(err)
	}
	homeAlias := filepath.Join(root, "home")
	if err := os.Symlink(realHome, homeAlias); err != nil {
		t.Skipf("cannot create home alias: %v", err)
	}
	t.Setenv("HOME", homeAlias)
	t.Setenv("USERPROFILE", homeAlias)
	t.Setenv("SKILLSHARE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "roaming"))

	legacy := filepath.Join(homeAlias, ".config", "skillshare", "config.yaml")
	old, err := projectConfigIdentity(legacy, "windows")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := projectConfigIdentity(ConfigPath(), "windows")
	if err != nil {
		t.Fatal(err)
	}
	if old != canonical {
		t.Fatalf("resolved legacy home identity %s != canonical %s", old, canonical)
	}
}

func TestLocalProjectsConfigLinksAndSnapshotIdentity(t *testing.T) {
	cfg, root, err := loadWithProjects(t, "", "app")
	if err != nil {
		t.Fatal(err)
	}
	path := ConfigPath()
	alias := filepath.Join(filepath.Dir(path), "alias.yaml")
	if err := os.Symlink(path, alias); err != nil {
		t.Skip(err)
	}
	a, err := LocalProjectsPath(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LocalProjectsPath(alias)
	if err != nil || a != b {
		t.Fatalf("aliases disagree: %s %s %v", a, b, err)
	}
	other := filepath.Join(filepath.Dir(path), "other.yaml")
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(other, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LocalProjectsPath(other)
	if err != nil || a == c {
		t.Fatalf("distinct destinations share identity: %v", err)
	}
	loaded, err := LoadReadOnlyPath(alias)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := PrepareLocalProjects(alias, loaded, map[string]ManagedProject{filepath.Join(root, "app"): {Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, alias); err != nil {
		t.Fatal(err)
	}
	if err := candidate.Check(); err == nil {
		t.Fatal("identical-byte retarget accepted")
	}
	// A caller may not reuse a snapshot belonging to a different config.
	if _, err := PrepareLocalProjects(other, cfg, cfg.Projects, ""); err == nil {
		t.Fatal("mismatched config snapshot accepted")
	}
}

func TestLocalProjectsRefuseRegistryRedirection(t *testing.T) {
	for _, namespace := range []bool{false, true} {
		t.Run(map[bool]string{false: "registry", true: "namespace"}[namespace], func(t *testing.T) {
			_, _, err := loadWithProjects(t, "", "app")
			if err != nil {
				t.Fatal(err)
			}
			path := ConfigPath()
			before, _ := os.ReadFile(path)
			local, err := LocalProjectsPath(path)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "external")
			link := local
			if namespace {
				link = filepath.Dir(local)
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(target, []byte("version: 1\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skip(err)
			}
			if _, err := LoadReadOnlyPath(path); err == nil {
				t.Fatal("symlinked registry accepted")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("read changed shared config")
			}
		})
	}
}
