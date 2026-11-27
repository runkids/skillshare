package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Mod a was docked first, so b's layout copy was made from a's and shows both panels.
func newHerdrChainedDocks(t *testing.T) (s *Service, home string, current *string) {
	t.Helper()
	s, herdr := newHerdrModsService(t, "a", "b")
	home, err := filepath.EvalSymlinks(herdr)
	if err != nil {
		t.Fatal(err)
	}
	block := `{"id":"main","type":"block","content":{"tab":"active"}}`
	dock := func(mod string) string {
		return `{"id":"` + herdrDockID(mod) + `","type":"block","content":{"mod":"` + mod + `"}}`
	}
	writePluginFile(t, home, "layouts/main.json", `{"id":"root","type":"row","children":[`+block+`]}`)
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ pkg, from, name, children string }{
		{"a", "main", "main-skillshare", block + "," + dock("a")},
		{"b", "main-skillshare", "main-skillshare-skillshare", block + "," + dock("a") + "," + dock("b")},
	} {
		data := `{"cloned_from":{"name":"` + c.from + `"},"id":"root","type":"row","children":[` + c.children + `]}`
		writePluginFile(t, home, "layouts/"+c.name+".json", data)
		b := d.state.Bindings[c.pkg]["herdr-app"]
		b.HerdrLayouts = []herdrLayoutReceipt{{Home: home, From: c.from, Name: c.name, Digest: hash([]byte(data)), DockMod: c.pkg}}
		d.state.Bindings[c.pkg]["herdr-app"] = b
	}
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
	active := "main-skillshare-skillshare"
	s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		switch {
		case bin == "herdr-app" && args[0] == "status":
			return json.Marshal(map[string]string{"layout": active})
		case bin == "herdr-app" && args[0] == "do" && args[1] == "layout.switch":
			var req map[string]string
			if err := json.Unmarshal([]byte(args[2]), &req); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(home, "layouts", req["name"]+".json")); err != nil {
				t.Fatalf("switched to a missing layout %s", req["name"])
			}
			active = req["name"]
			return nil, nil
		}
		t.Fatalf("unexpected command %s %q", bin, args)
		return nil, nil
	}
	return s, home, &active
}

// Every mod the window's layout shows must still be installed.
func assertHerdrWindowShowsInstalledMods(t *testing.T, home, active string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "layouts", active+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	walkHerdrNodes(root, func(node map[string]any) {
		content, _ := node["content"].(map[string]any)
		if mod, _ := content["mod"].(string); mod != "" {
			if _, err := os.Stat(filepath.Join(home, "mods", mod, "main.js")); err != nil {
				t.Errorf("window layout %s still shows uninstalled mod %s", active, mod)
			}
		}
	})
}

func TestHerdrLayoutViewFollowsChainedCopies(t *testing.T) {
	const (
		original = "main"
		copyA    = "main-skillshare"
		copyB    = "main-skillshare-skillshare"
	)
	type step struct {
		action string
		names  []string
		want   string
		only   string // sync only this package
		// action "move" puts the window on want by hand; "sync" only syncs.
	}
	for _, tc := range []struct {
		name  string
		steps []step
	}{
		{"disable a, then enable a", []step{{"disable", []string{"a"}, original, ""}, {"enable", []string{"a"}, copyB, ""}}},
		{"disable b, then enable b", []step{{"disable", []string{"b"}, copyA, ""}, {"enable", []string{"b"}, copyB, ""}}},
		{"disable both, then enable both", []step{{"disable", []string{"a", "b"}, original, ""}, {"enable", []string{"a", "b"}, copyB, ""}}},
		{"disable both, enable b, then a", []step{{"disable", []string{"a", "b"}, original, ""}, {"enable", []string{"b"}, original, ""}, {"enable", []string{"a"}, copyB, ""}}},
		{"disable both, enable b, then sync only a", []step{{"disable", []string{"a", "b"}, original, ""}, {"enable", []string{"b"}, original, ""}, {"enable", []string{"a"}, copyB, "a"}}},
		{"disable both, enable a, then b", []step{{"disable", []string{"a", "b"}, original, ""}, {"enable", []string{"a"}, copyA, ""}, {"enable", []string{"b"}, copyB, ""}}},
		{"disable a, then b, enable a, then b", []step{{"disable", []string{"a"}, original, ""}, {"disable", []string{"b"}, original, ""}, {"enable", []string{"a"}, copyA, ""}, {"enable", []string{"b"}, copyB, ""}}},
		{"disable a, then b, enable a, sync, enable b", []step{{"disable", []string{"a"}, original, ""}, {"disable", []string{"b"}, original, ""}, {"enable", []string{"a"}, copyA, ""}, {"sync", nil, copyA, ""}, {"enable", []string{"b"}, copyB, ""}}},
		{"disable a, then b, enable both", []step{{"disable", []string{"a"}, original, ""}, {"disable", []string{"b"}, original, ""}, {"enable", []string{"a", "b"}, copyB, ""}}},
		{"disable b, then a, enable both", []step{{"disable", []string{"b"}, copyA, ""}, {"disable", []string{"a"}, original, ""}, {"enable", []string{"a", "b"}, copyB, ""}}},
		{"disable b", []step{{"disable", []string{"b"}, copyA, ""}}},
		{"disable both, enable a", []step{{"disable", []string{"a", "b"}, original, ""}, {"enable", []string{"a"}, copyA, ""}}},
		{"disable both, enable b, move to a's copy, enable a, then cycle b", []step{{"disable", []string{"a", "b"}, original, ""}, {"enable", []string{"b"}, original, ""}, {"move", nil, copyA, ""}, {"enable", []string{"a"}, copyA, ""}, {"disable", []string{"b"}, copyA, ""}, {"enable", []string{"b"}, copyA, ""}}},
		{"disable b, move to main, cycle a", []step{{"disable", []string{"b"}, copyA, ""}, {"move", nil, original, ""}, {"disable", []string{"a"}, original, ""}, {"enable", []string{"a"}, original, ""}}},
		{"remove a", []step{{"remove", []string{"a"}, original, ""}}},
		{"remove b", []step{{"remove", []string{"b"}, copyA, ""}}},
		{"remove both", []step{{"remove", []string{"a", "b"}, original, ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, home, active := newHerdrChainedDocks(t)
			for i, st := range tc.steps {
				if st.action == "move" {
					*active = st.want
					continue
				}
				if st.action != "sync" {
					applyPluginRequest(t, s, Request{Action: st.action, Names: st.names})
				}
				if st.action != "remove" {
					applyPluginRequest(t, s, Request{Action: "sync", Name: st.only})
				}
				if *active != st.want {
					t.Fatalf("step %d (%s %v): window on %s, want %s", i+1, st.action, st.names, *active, st.want)
				}
				assertHerdrWindowShowsInstalledMods(t, home, *active)
			}
			// A window the user moved stays put: no stale return is left to replay.
			*active = original
			applyPluginRequest(t, s, Request{Action: "sync"})
			if *active != original {
				t.Fatalf("a later sync moved the window to %s", *active)
			}
		})
	}
}

// A copy suspended before receipts recorded where the window left returns to that copy.
func TestHerdrLayoutViewReturnsLegacySuspendedCopy(t *testing.T) {
	s, _, active := newHerdrChainedDocks(t)
	applyPluginRequest(t, s, Request{Action: "disable", Name: "a"})
	applyPluginRequest(t, s, Request{Action: "sync"})
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	b := d.state.Bindings["a"]["herdr-app"]
	b.HerdrLayouts[0].Left = ""
	d.state.Bindings["a"]["herdr-app"] = b
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "enable", Name: "a"})
	applyPluginRequest(t, s, Request{Action: "sync"})
	if *active != "main-skillshare" {
		t.Fatalf("window on %s, want main-skillshare", *active)
	}
}
