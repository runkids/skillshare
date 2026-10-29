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
