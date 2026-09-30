package mcp

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mutateSaved(t *testing.T, s *Service, m Mutation) *Source {
	t.Helper()
	if _, err := s.Mutate(m, "", false); err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestMutateSettingsSetsTheGlobalDefaults(t *testing.T) {
	s, _ := projectsService(t, projectsConfig)
	// directTools is ignored since 0.23.0; a dashboard that still sends it can save.
	source := mutateSaved(t, s, Mutation{Settings: &Settings{Targets: []string{"opencode", "pi"}, DirectTools: "search"}})
	if !reflect.DeepEqual(source.Targets, []string{"opencode", "pi"}) {
		t.Fatalf("targets %v", source.Targets)
	}
	data, _ := os.ReadFile(s.ConfigPath)
	if strings.Contains(string(data), "directTools") {
		t.Fatalf("directTools saved:\n%s", data)
	}
}

func TestMutateProjectAddsARootUnderTheKeyAsTyped(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	source := mutateSaved(t, s, Mutation{Project: "~/projC", Settings: &Settings{Targets: []string{"opencode"}}})
	if got := source.Projects[filepath.Join(tmp, "projC")].Targets; !reflect.DeepEqual(got, []string{"opencode"}) {
		t.Fatalf("projC targets %v", got)
	}
	data, _ := os.ReadFile(s.ConfigPath)
	if !strings.Contains(string(data), "~/projC:") {
		t.Fatalf("key was rewritten:\n%s", data)
	}
}

func TestMutateProjectServerIsSavedInThatRootOnly(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	root := filepath.Join(tmp, "projA")
	source := mutateSaved(t, s, Mutation{Project: root, Name: "shared", Server: &Server{Disabled: true, Targets: []string{"opencode"}}})
	if !source.Projects[root].Servers["shared"].Disabled || source.Servers["shared"].Disabled {
		t.Fatalf("project %+v, global %+v", source.Projects[root].Servers, source.Servers)
	}
	source = mutateSaved(t, s, Mutation{Project: root, Name: "shared", Remove: true})
	if _, ok := source.Projects[root].Servers["shared"]; ok {
		t.Fatal("shared is still in projA")
	}
	if _, ok := source.Projects[root].Servers["docs"]; !ok {
		t.Fatal("docs was dropped from projA")
	}
}

func TestMutateProjectRemoveDropsTheRoot(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	source := mutateSaved(t, s, Mutation{Project: filepath.Join(tmp, "projB"), Remove: true})
	if len(source.Projects) != 1 {
		t.Fatalf("projects %v", source.Projects)
	}
}

func TestMutateProjectLeavesOtherRootsAsWritten(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  projects:
    $TMP/projA:
      servers:
        docs: &docs
          command: docs-server
          targets: [cursor]
    $TMP/projB:
      servers:
        docs: *docs
`)
	mutateSaved(t, s, Mutation{Project: filepath.Join(tmp, "projC"), Settings: &Settings{}})
	data, _ := os.ReadFile(s.ConfigPath)
	if !strings.Contains(string(data), "docs: *docs") {
		t.Fatalf("alias was written out:\n%s", data)
	}
}

func TestMutateProjectRejected(t *testing.T) {
	s, tmp := projectsService(t, projectsConfig)
	for name, m := range map[string]Mutation{
		"relative root": {Project: "work/p1", Settings: &Settings{}},
		"unknown root":  {Project: filepath.Join(tmp, "nope"), Remove: true},
		"bad target":    {Project: filepath.Join(tmp, "projA"), Replace: true, Settings: &Settings{Targets: []string{"nope!"}}},
		"listed twice":  {Project: filepath.Join(tmp, "projA"), Settings: &Settings{}},
	} {
		if _, err := s.Mutate(m, "", false); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
}
