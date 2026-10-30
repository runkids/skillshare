package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLegacyRemovalDoesNotRepeatAfterManualReinstall(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := native.service(t)
			if project {
				s.ProjectRoot = t.TempDir()
				s.ConfigPath = filepath.Join(s.ProjectRoot, ".skillshare", "config.yaml")
				native.installed["claude"][0].Scope = "project"
				native.installed["claude"][0].ProjectPath = s.ProjectRoot
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team, pending: remove}\n")
			same := unchanged(t, s.ConfigPath)
			applyPluginRequest(t, s, Request{Action: "sync"})
			if len(native.installed["claude"]) != 0 {
				t.Fatal("legacy removal did not finish")
			}
			scope := "user"
			if project {
				scope = "project"
			}
			native.installed["claude"] = []Installed{{ID: "demo@team", Scope: scope, ProjectPath: s.ProjectRoot, Installed: true}}
			native.commands = nil
			for range 2 {
				applyPluginRequest(t, s, Request{Action: "sync"})
			}
			same()
			if len(native.installed["claude"]) != 1 || len(native.commands) != 0 {
				t.Fatalf("manual reinstall removed again: %v %v", native.installed, native.commands)
			}
		})
	}
}

func TestImportRejectsUnrelatedSourceRecipe(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "unrelated@team", Scope: "user", Installed: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    review:\n      source: "+source+"\n      plugin: demo\n      bindings:\n        claude: {}\n")
	same := unchanged(t, s.ConfigPath)
	_, err := s.Preview(context.Background(), Request{Action: "import", From: "claude", Plugin: "unrelated@team", Name: "review"})
	if key, _ := ErrorKey(err); key != "plugins.error.importRecipeMismatch" {
		t.Fatalf("unrelated import accepted: %v", err)
	}
	same()
}

func TestLocalImportSurvivesSharedRecipeChanges(t *testing.T) {
	for _, kind := range []string{"source", "ref"} {
		t.Run(kind, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := native.service(t)
			source := fixture(t)
			if kind == "ref" {
				source = "https://fixture.invalid/imported.git"
			}
			raw := "plugins:\n  version: 2\n  packages:\n    review:\n      source: " + source + "\n      plugin: demo\n      bindings:\n        claude: {}\n"
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), raw)
			applyPluginRequest(t, s, Request{Action: "import", From: "claude", Plugin: "demo@team", Name: "review"})
			if kind == "ref" {
				raw = strings.Replace(raw, "      plugin: demo", "      source_ref: v2\n      plugin: demo", 1)
			} else {
				raw = strings.Replace(raw, source, fixture(t), 1)
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), raw)
			same := unchanged(t, s.ConfigPath)
			native.commands = nil
			p, err := s.Preview(context.Background(), Request{Action: "sync"})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Changes) != 1 || p.Changes[0].Action != "noop" || p.Changes[0].MessageKey != "plugins.note.importRecipeChanged" || !strings.Contains(p.Changes[0].Message, "plugin unimport") {
				t.Fatalf("import replaced or missing guidance: %+v", p.Changes)
			}
			result, err := s.Apply(context.Background(), Request{Action: "sync"}, p.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results) != 1 || result.Results[0].MessageKey != "plugins.note.importRecipeChanged" {
				t.Fatalf("normal sync dropped guidance: %+v", result)
			}
			same()
			if len(native.commands) != 0 || len(native.installed["claude"]) != 1 {
				t.Fatalf("import changed native installation: %v", native.commands)
			}
		})
	}

}

func TestLegacyReadHintPersistenceRequiresOperationLock(t *testing.T) {
	native := &fakeAgents{}
	s := native.service(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.statePath()); !os.IsNotExist(err) {
		t.Fatalf("read wrote local state while operation lock held: %v", err)
	}
	unlock()
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.statePath()); err != nil {
		t.Fatalf("next unlocked read did not persist hints: %v", err)
	}
}

func TestConcurrentLegacyReadersPersistHintsWithoutConflict(t *testing.T) {
	native := &fakeAgents{}
	s := native.service(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	start := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, err := s.Packages(); errs <- err }()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.statePath()); err != nil {
		t.Fatal(err)
	}
}
