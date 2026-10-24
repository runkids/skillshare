package projects

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

func serviceFixture(t *testing.T) (*Service, *config.Config, string, string, []byte) {
	t.Helper()
	path, cfg, dir, original := projectCandidateFixture(t, "  $ROOT/app:\n    targets: [claude, codex]\n    skills: {}\n    agents: {}\n")
	root := filepath.Join(dir, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	// Expansion excludes missing roots, so reload after creating the folder.
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Source = filepath.Join(dir, "source")
	if err := os.MkdirAll(cfg.Source, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	return NewService(path, func() (*config.Config, error) { return cfg, nil }), cfg, path, root, original
}
func TestServiceRemove_InvalidCandidateBeforeDetach(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	cfg.Projects[filepath.Join(filepath.Dir(root), "missing")] = config.ManagedProject{Targets: []string{"claude"}, Skills: &config.ResourceTargetConfig{Include: []string{"["}}}
	calls := 0
	s.DetachSkillsTarget = func(string, ssync.DetachOptions) (ssync.DetachReport, error) {
		calls++
		return ssync.DetachReport{}, nil
	}
	_, err := s.Remove(root, RemoveOptions{})
	if err == nil {
		t.Fatal("invalid removal candidate accepted")
	}
	if calls != 0 {
		t.Fatalf("invalid removal detached %d targets before validation", calls)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("invalid removal changed disk")
	}
	if _, ok := cfg.Projects[root]; !ok {
		t.Fatal("invalid removal changed memory")
	}
}
func TestServiceRemove_LaterFailurePreservesReport(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	manifests := serviceTargetManifests(t, cfg)
	calls := 0
	s.DetachSkillsTarget = func(path string, opts ssync.DetachOptions) (ssync.DetachReport, error) {
		calls++
		if calls == 2 {
			return ssync.DetachReport{Retained: []string{path}}, errors.New("denied")
		}
		return ssync.DetachSkillsTarget(path, opts)
	}
	result, err := s.Remove(root, RemoveOptions{})
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want later failure: %v", err)
	}
	if result.Registration != "kept" || result.Detach == nil || len(result.Detach.Removed) != 1 || len(result.Detach.Retained) != 1 {
		t.Fatalf("completed detach report lost: %+v", result)
	}

	if _, err := os.Stat(result.Detach.Removed[0]); !os.IsNotExist(err) {
		t.Fatalf("first detach did not remove manifest: %v", err)
	}
	remaining := 0
	for _, manifest := range manifests {
		if _, err := os.Stat(manifest); err == nil {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("later target changed after failure: %d manifests remain", remaining)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("partial removal changed disk")
	}
	if _, ok := cfg.Projects[root]; !ok {
		t.Fatal("partial removal changed memory")
	}
}
func TestServiceRemove_SaveFailureKeepsMemory(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	configDir := filepath.Join(filepath.Dir(path), "config-dir")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(s.path, original, 0644); err != nil {
		t.Fatal(err)
	}
	moved := configDir + "-moved"
	manifests := serviceTargetManifests(t, cfg)
	calls := 0
	s.DetachSkillsTarget = func(path string, opts ssync.DetachOptions) (ssync.DetachReport, error) {
		detached, err := ssync.DetachSkillsTarget(path, opts)
		if err != nil {
			return detached, err
		}
		calls++
		if calls == 1 {
			if err := os.Rename(configDir, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configDir, []byte("blocked"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		return detached, nil
	}
	result, err := s.Remove(root, RemoveOptions{})
	if !errors.Is(err, ErrSave) {
		t.Fatalf("want persistence failure: %v", err)
	}
	if result.Registration != "kept" || result.Detach == nil || len(result.Detach.Removed) != 2 {
		t.Fatalf("save failure lost completed detach: %+v", result)
	}

	for _, manifest := range manifests {
		if _, err := os.Stat(manifest); !os.IsNotExist(err) {
			t.Fatalf("completed detach not retained after save failure: %v", err)
		}
	}
	if _, ok := cfg.Projects[root]; !ok {
		t.Fatal("save failure removed in-memory registration")
	}
	if got, _ := os.ReadFile(filepath.Join(moved, "config.yaml")); !bytes.Equal(got, original) {
		t.Fatal("save failure changed disk")
	}
}
func TestServiceRemove_AgentFilesUnchanged(t *testing.T) {
	s, cfg, _, root, _ := serviceFixture(t)
	agents := filepath.Join(root, ".claude", "agents", "review.md")
	if err := os.MkdirAll(filepath.Dir(agents), 0755); err != nil {
		t.Fatal(err)
	}
	content := []byte("agent content\n")
	if err := os.WriteFile(agents, content, 0644); err != nil {
		t.Fatal(err)
	}
	result, err := s.Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Registration != "removed" || len(result.Expanded.Projects) != 0 || len(cfg.Projects) != 1 {
		t.Fatalf("removal state: %+v, caller projects %v", result, cfg.Projects)
	}
	if got, _ := os.ReadFile(agents); !bytes.Equal(got, content) {
		t.Fatal("removal changed agent files")
	}
}

func TestServicePut_NoOpAndPreviewDoNotWrite(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	old := cfg.Projects[root]
	result, err := s.Put(root, old, PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Affected) != 0 || result.Item == nil || result.Expanded == cfg {
		t.Fatalf("no-op result: %+v", result)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("no-op wrote config")
	}
	next := old
	next.Name = "renamed"
	preview, err := s.Put(root, next, PutOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Affected) != 1 || preview.Item.Name != "renamed" || preview.Expanded.Targets["renamed@claude"].ProjectRoot() != root {
		t.Fatalf("preview not expanded from candidate: %+v", preview)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("preview wrote config")
	}
	if cfg.Projects[root].Name != "" {
		t.Fatal("preview changed caller")
	}
	applied, err := s.Put(root, next, PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Item.Name != preview.Item.Name || applied.Expanded.Targets["renamed@claude"].ProjectRoot() != root {
		t.Fatalf("apply diverged from preview: %+v", applied)
	}
	disk, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if disk.Projects[root].Name != "renamed" || cfg.Projects[root].Name != "" {
		t.Fatal("put did not isolate caller and persist candidate")
	}
}

func TestServiceRemove_PreviewAndLeftoverRoots(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	mcpRoot := root + string(filepath.Separator) + "."
	hooksRoot := root + string(filepath.Separator)
	var extra struct {
		MCP   *config.MCPConfig `yaml:"mcp"`
		Hooks yaml.Node         `yaml:"hooks"`
	}
	if err := yaml.Unmarshal([]byte("mcp:\n  projects:\n    "+mcpRoot+": {servers: {}}\nhooks:\n  projects:\n    "+hooksRoot+": {entries: {}}\n"), &extra); err != nil {
		t.Fatal(err)
	}
	cfg.MCP = extra.MCP
	cfg.Hooks = extra.Hooks
	calls := 0
	s.DetachSkillsTarget = func(path string, opts ssync.DetachOptions) (ssync.DetachReport, error) {
		calls++
		if !opts.DryRun {
			t.Fatal("preview detached files")
		}
		return ssync.DetachReport{Retained: []string{path}}, nil
	}
	result, err := s.Remove(root, RemoveOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.Registration != "kept" || len(result.Detach.Retained) != 2 || len(result.Warnings) != 2 || result.Warnings[0] != "mcp.projects still contains root "+mcpRoot || result.Warnings[1] != "hooks.projects still contains root "+hooksRoot {
		t.Fatalf("preview report: %+v", result)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("preview wrote config")
	}
	if len(cfg.Projects) != 1 || len(result.Expanded.Projects) != 0 {
		t.Fatal("preview state incorrect")
	}
}

func TestServiceList(t *testing.T) {
	s, _, _, root, _ := serviceFixture(t)
	listed, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Root != root || listed[0].Name != "app" || listed[0].Missing {
		t.Fatalf("list: %+v", listed)
	}
	if listed[0].Skills == nil || listed[0].Agents == nil {
		t.Fatalf("list resources: %+v", listed[0])
	}
}

func TestServiceShow(t *testing.T) {
	s, _, _, root, _ := serviceFixture(t)
	shown, err := s.Show("app")
	if err != nil {
		t.Fatal(err)
	}
	if shown.Root != root || shown.Skills == nil || shown.Agents == nil {
		t.Fatalf("show: %+v", shown)
	}
	_, err = s.Show("unknown")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func serviceTargetManifests(t *testing.T, cfg *config.Config) []string {
	t.Helper()
	var paths []string
	for _, target := range cfg.Targets {
		path := filepath.Join(target.SkillsConfig().Path, ssync.ManifestFile)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestServicePut_ResolvedDeclaredKeyWithRootAliases(t *testing.T) {
	s, cfg, path, root, _ := serviceFixture(t)
	entry := cfg.Projects[root]
	entry.Name = "primary"
	entry.Targets = []string{"claude"}
	cfg.Projects[root] = entry
	alias := root + string(filepath.Separator) + "."
	other := entry
	other.Name = "secondary"
	cfg.Projects[alias] = other
	candidate, err := config.PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Write(); err != nil {
		t.Fatal(err)
	}
	s.load = config.LoadReadOnly
	entry.Skills = &config.ResourceTargetConfig{Mode: "copy"}
	result, err := s.Put(root, entry, PutOptions{})
	if err != nil {
		t.Fatalf("resolved declared key rejected: %v", err)
	}
	if result.Item.Root != root || len(result.Affected) != 1 || result.Affected[0] != root {
		t.Fatalf("wrong declaration edited: %+v", result)
	}
	removed, err := s.Remove(alias, RemoveOptions{DryRun: true})
	if err != nil || removed.Item.Root != alias {
		t.Fatalf("resolved key rejected by Remove: %+v %v", removed, err)
	}
	_, err = s.Put(root, entry, PutOptions{Create: true})
	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) || len(ambiguous.Candidates) != 2 {
		t.Fatalf("Create did not check root identity: %v", err)
	}
}

// Windows CI runs this with native case-folding path identities.
func TestServiceExactKeysWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows path identity")
	}
	s, cfg, path, root, _ := serviceFixture(t)
	entry := cfg.Projects[root]
	entry.Name = "primary"
	entry.Targets = []string{"claude"}
	cfg.Projects[root] = entry
	alias := strings.ToUpper(root)
	if alias == root {
		alias = strings.ToLower(root)
	}
	other := entry
	other.Name = "secondary"
	cfg.Projects[alias] = other
	c, err := config.PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(); err != nil {
		t.Fatal(err)
	}
	s.load = config.LoadReadOnly
	entry.Skills = &config.ResourceTargetConfig{Mode: "copy"}
	put, err := s.Put(root, entry, PutOptions{})
	if err != nil || put.Item.Root != root {
		t.Fatalf("exact Put: %+v %v", put, err)
	}
	removed, err := s.Remove(alias, RemoveOptions{DryRun: true})
	if err != nil || removed.Item.Root != alias || len(removed.Expanded.Projects) != 1 {
		t.Fatalf("exact Remove: %+v %v", removed, err)
	}
	loaded, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	_, err = FindRoot(loaded.Projects, root, filepath.Dir(root), filepath.Dir(root))
	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) || len(ambiguous.Candidates) != 2 {
		t.Fatalf("root ambiguity lost: %v", err)
	}
}
