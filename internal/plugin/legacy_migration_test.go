package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyPendingRemovalRetriesAfterMigration(t *testing.T) {
	for _, whole := range []bool{false, true} {
		name := "target"
		if whole {
			name = "package"
		}
		t.Run(name, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}, "codex": {{ID: "demo@team", Installed: true}}}}
			s := native.service(t)
			codex := ""
			if !whole {
				codex = "        codex: {id: demo@team}\n"
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team, pending: remove}\n"+codex+"    other:\n      bindings:\n        claude: {id: other@team}\n")
			// An unrelated declaration edit upgrades the file before the retry.
			applyPluginRequest(t, s, Request{Action: "disable", Name: "other", Targets: []string{"claude"}})
			raw, err := os.ReadFile(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.Preview(context.Background(), Request{Action: "sync", Name: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, c := range plan.Changes {
				if c.Target == "claude" && c.Action == "remove" {
					found = true
				}
			}
			if !found {
				t.Fatalf("pending removal was lost: %+v", plan.Changes)
			}
			applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			if len(native.installed["claude"]) != 0 {
				t.Fatal("removed plugin remains installed")
			}
			if !whole && len(native.installed["codex"]) != 1 {
				t.Fatal("removed another target")
			}
			after, _ := os.ReadFile(s.ConfigPath)
			if string(after) != string(raw) {
				t.Fatal("retry rewrote shared declarations")
			}
		})
	}
}

func TestLegacyReadPersistsEvidenceBeforeRemoteMigration(t *testing.T) {
	for _, read := range []string{"packages", "preview"} {
		t.Run(read, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := native.service(t)
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
			same := unchanged(t, s.ConfigPath)
			if read == "packages" {
				if _, err := s.Packages(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := s.Preview(context.Background(), Request{Action: "sync"}); err != nil {
					t.Fatal(err)
				}
			}
			same()
			// Another machine migrates the shared file before this host applies anything.
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        claude: {imported: true}\n")
			restarted := *s
			p, err := restarted.Preview(context.Background(), Request{Action: "sync"})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Changes) != 1 || p.Changes[0].Action != "noop" || p.Changes[0].ID != "demo@team" {
				t.Fatalf("read lost local migration evidence: %+v", p.Changes)
			}
		})
	}
}

func TestSharedImportRequiredPlanHasLocalizedGuidance(t *testing.T) {
	s := (&fakeAgents{}).service(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        claude: {imported: true}\n")
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.localImportRequired" {
		t.Fatalf("unlocalized plan: %+v", p.Changes)
	}
}
