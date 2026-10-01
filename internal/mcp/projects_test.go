package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func projectsService(t *testing.T, config string) (*Service, string) {
	t.Helper()
	tmp := t.TempDir()
	s := &Service{ConfigPath: filepath.Join(tmp, "config.yaml"), Home: tmp, StateDir: filepath.Join(tmp, "state")}
	config = strings.ReplaceAll(config, "$TMP", tmp)
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return s, tmp
}

func applyProjects(t *testing.T, s *Service) *Plan {
	t.Helper()
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Blocked {
		t.Fatalf("blocked: %+v", plan.Changes)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	return plan
}

const projectsConfig = `mcp:
  servers:
    shared:
      command: echo
      targets: [cursor, opencode]
  projects:
    $TMP/projA:
      servers:
        docs:
          command: docs-server
          targets: [cursor]
    $TMP/projB:
      targets: [opencode]
      servers:
        shared:
          disabled: true
`

func TestProjectsSyncEveryRootInOnePlan(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	for file, want := range map[string]string{
		".cursor/mcp.json":       `"shared"`,
		"projA/.cursor/mcp.json": `"docs-server"`,
		"projB/opencode.json":    `"enabled": false`,
	} {
		data, err := os.ReadFile(filepath.Join(tmp, file))
		if err != nil || !strings.Contains(string(data), want) {
			t.Fatalf("%s: want %s, got %s (%v)", file, want, data, err)
		}
	}
	// A second run must not see one root's entries as leftovers of another.
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Changes {
		if c.Action != "unchanged" {
			t.Fatalf("second preview: %s %s %s", c.Action, c.Name, c.Path)
		}
	}
}

func TestProjectsRemovedRootIsCleaned(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	data, _ := os.ReadFile(s.ConfigPath)
	trimmed := string(data)[:strings.Index(string(data), "    "+tmp+"/projB")]
	if err := os.WriteFile(s.ConfigPath, []byte(trimmed), 0600); err != nil {
		t.Fatal(err)
	}
	applyProjects(t, s)
	got, _ := os.ReadFile(filepath.Join(tmp, "projB", "opencode.json"))
	if strings.Contains(string(got), "shared") {
		t.Fatalf("projB entry kept: %s", got)
	}
	if kept, _ := os.ReadFile(filepath.Join(tmp, "projA", ".cursor", "mcp.json")); !strings.Contains(string(kept), "docs-server") {
		t.Fatalf("projA entry lost: %s", kept)
	}
}

func TestLegacyRemovedProjectIsParkedWithRecovery(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	state, _, err := s.loadLedger()
	if err != nil {
		t.Fatal(err)
	}
	for key, owned := range state.Entries {
		owned.Root = nil
		state.Entries[key] = owned
	}
	if err := writeJSONFile(s.statePath(), state); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := config[:strings.Index(string(config), "    "+tmp+"/projB")]
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(trimmed)
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, "projB", "opencode.json")
	if changeFor(p, path, "shared") != nil || !strings.Contains(strings.Join(p.Notices, "\n"), "no saved scope") {
		t.Fatalf("legacy project needs safe recovery: changes=%+v notices=%v", p.Changes, p.Notices)
	}
	applyProjects(t, s)
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "shared") {
		t.Fatalf("legacy project was pruned: %s (%v)", data, err)
	}
	write(config)
	applyProjects(t, s)
	write(trimmed)
	applyProjects(t, s)
	if data, err := os.ReadFile(path); err != nil || strings.Contains(string(data), "shared") {
		t.Fatalf("project cleanup did not resume: %s (%v)", data, err)
	}
}

func TestLegacyProjectScopeBackfilledOnPiSettingsUpdate(t *testing.T) {
	s := testService(t)
	root := filepath.Join(s.Home, "project")
	config := "mcp:\n  projects:\n    " + root + ":\n      targets: [pi]\n      servers:\n        docs:\n          command: docs-tool\n          piOptions: {timeout: 10}\n"
	write := func(config string) {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(config)
	applyProjects(t, s)
	state, _, err := s.loadLedger()
	if err != nil {
		t.Fatal(err)
	}
	for key, owned := range state.Entries {
		owned.Root = nil
		state.Entries[key] = owned
	}
	if err := writeJSONFile(s.statePath(), state); err != nil {
		t.Fatal(err)
	}
	write(strings.ReplaceAll(config, "timeout: 10", "timeout: 20"))
	applyProjects(t, s)
	write("mcp:\n  servers: {}\n")
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if c := changeFor(p, filepath.Join(root, ".pi", "mcp.json"), "docs"); c == nil || c.Action != "remove" {
		t.Fatalf("settings update did not record project scope: changes=%+v notices=%v", p.Changes, p.Notices)
	}
}

func TestProjectImportSavePreservesOwnershipScope(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	applyProjects(t, s)
	root := filepath.Join(tmp, "projA")
	path := filepath.Join(root, ".cursor", "mcp.json")
	server := Server{Command: "docs-server", Targets: []string{"cursor"}}
	if _, err := s.Mutate(Mutation{
		Project: root, Name: "docs", Server: &server, Replace: true,
		Resolutions: []Resolution{{Target: "cursor", Name: "docs", Action: "adopt"}},
	}, "", false); err != nil {
		t.Fatal(err)
	}
	state, _, err := s.loadLedger()
	if err != nil {
		t.Fatal(err)
	}
	if owned := state.Entries[ownershipKey("cursor", path, "docs")]; owned.Root == nil || *owned.Root != root {
		t.Errorf("save lost the adopted entry's project scope: %+v", owned)
	}
	if _, err := s.Mutate(Mutation{Project: root, Remove: true}, "", false); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if c := changeFor(p, path, "docs"); c == nil || c.Action != "remove" {
		t.Fatalf("adopted project cleanup was parked: changes=%+v notices=%v", p.Changes, p.Notices)
	}
}

func TestProjectsRejected(t *testing.T) {
	for name, tc := range map[string]struct{ config, want string }{
		"relative root":  {"mcp:\n  projects:\n    work/p1:\n      servers: {}\n", "absolute"},
		"unknown field":  {"mcp:\n  projects:\n    $TMP/p1:\n      path: x\n", "field path not found"},
		"codex disabled": {"mcp:\n  projects:\n    $TMP/p1:\n      servers:\n        x:\n          disabled: true\n          targets: [codex]\n", "stops loading its whole config"},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := projectsService(t, tc.config)
			if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
	t.Run("inside a project config", func(t *testing.T) {
		s, tmp := projectsService(t, "mcp:\n  projects:\n    $TMP/p1:\n      servers: {}\n")
		s.ProjectRoot = tmp
		if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "global") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestProjectsReuseAnchorFromGlobalServers(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  servers:
    docs: &docs
      command: docs-server
      targets: [cursor]
  projects:
    $TMP/projA:
      servers:
        docs: *docs
`)
	applyProjects(t, s)
	data, err := os.ReadFile(filepath.Join(tmp, "projA", ".cursor", "mcp.json"))
	if err != nil || !strings.Contains(string(data), "docs-server") {
		t.Fatalf("aliased server not written: %s (%v)", data, err)
	}
}

func TestSavingServersKeepsProjectAliasesValid(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  servers:
    docs: &docs
      command: docs-server
      targets: [cursor]
  projects:
    $TMP/projA:
      servers:
        docs: *docs
`)
	added := &Server{Command: "echo", Targets: []string{"cursor"}}
	if _, err := s.Mutate(Mutation{Name: "added", Server: added}, "", false); err != nil {
		t.Fatal(err)
	}
	applyProjects(t, s)
	data, err := os.ReadFile(filepath.Join(tmp, "projA", ".cursor", "mcp.json"))
	if err != nil || !strings.Contains(string(data), "docs-server") {
		t.Fatalf("project entry lost after saving servers: %s (%v)", data, err)
	}
}

func TestProjectsRootExpandsHomeWithThePlatformSeparator(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("'~"+string(filepath.Separator)+"work':\n  servers: {}\n"), &node); err != nil {
		t.Fatal(err)
	}
	projects, err := ParseProjects(node.Content[0])
	if _, ok := projects[filepath.Join(home, "work")]; err != nil || !ok {
		t.Fatalf("~ with the platform separator was not expanded: %v %v", projects, err)
	}
}

func TestApplyProjectWritesOnlyThatRoot(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	root := filepath.Join(tmp, "projA")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyProject(plan.Revision, root); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, ".cursor", "mcp.json")); err != nil || !strings.Contains(string(data), "docs-server") {
		t.Fatalf("projA not written: %s (%v)", data, err)
	}
	for _, file := range []string{".cursor/mcp.json", "projB/opencode.json"} {
		if _, err := os.Stat(filepath.Join(tmp, file)); !os.IsNotExist(err) {
			t.Fatalf("%s was written", file)
		}
	}
	after, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range after.Changes {
		if pending := c.Action != "unchanged"; pending == (c.Root == root) {
			t.Fatalf("after project apply: %s %s %s (root %q)", c.Action, c.Name, c.Path, c.Root)
		}
	}
}

func TestApplyProjectKeepsClaudeUserServersPending(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  servers:
    shared:
      command: echo
      targets: [claude]
  projects:
    $TMP/projA:
      targets: [claude]
      servers:
        shared:
          disabled: true
`)
	root := filepath.Join(tmp, "projA")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyProject(plan.Revision, root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"shared"`) || strings.Contains(string(data), "echo") {
		t.Fatalf("want only projA's off list in ~/.claude.json, got %s", data)
	}
}

func TestApplyProjectRejectsUnknownRoot(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyProject(plan.Revision, filepath.Join(tmp, "nope")); !errors.Is(err, ErrUnknownProject) {
		t.Fatalf("got %v", err)
	}
}
