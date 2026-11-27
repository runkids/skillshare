package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// newHerdrModsService adds one Herdr mod per name from a single source.
func newHerdrModsService(t *testing.T, names ...string) (*Service, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override Windows profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.Mkdir(herdr, 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for _, name := range names {
		writePluginFile(t, source, "mods/"+name+"/main.js", name)
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Plugins: names, Targets: []string{"herdr-app"}})
	return s, herdr
}

func TestNamesDisableAndEnableSeveralMods(t *testing.T) {
	s, _ := newHerdrModsService(t, "a", "b", "c")
	selected := func() map[string]bool {
		t.Helper()
		d, err := s.load()
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for name, pack := range d.intent {
			sync := pack.Bindings["herdr-app"].Sync
			got[name] = sync == nil || *sync
		}
		return got
	}
	applyPluginRequest(t, s, Request{Action: "disable", Names: []string{"a", "b"}})
	if got := selected(); got["a"] || got["b"] || !got["c"] {
		t.Fatalf("after disable: %v", got)
	}
	applyPluginRequest(t, s, Request{Action: "enable", Names: []string{"a", "b"}})
	if got := selected(); !got["a"] || !got["b"] || !got["c"] {
		t.Fatalf("after enable: %v", got)
	}
}

func TestNamesRemoveSeveralMods(t *testing.T) {
	s, herdr := newHerdrModsService(t, "a", "b", "c")
	applyPluginRequest(t, s, Request{Action: "remove", Names: []string{"a", "b"}})
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.intent["a"]; ok {
		t.Fatal("a is still declared")
	}
	if _, ok := d.intent["b"]; ok {
		t.Fatal("b is still declared")
	}
	if _, ok := d.intent["c"]; !ok {
		t.Fatal("c was removed with the others")
	}
	for name, want := range map[string]bool{"a": false, "b": false, "c": true} {
		_, err := os.Stat(filepath.Join(herdr, "mods", name, "main.js"))
		if (err == nil) != want {
			t.Fatalf("mods/%s installed = %v, want %v", name, err == nil, want)
		}
	}
}

func TestNamesRejectsWhatItCannotPick(t *testing.T) {
	s, _ := newHerdrModsService(t, "a", "b")
	for label, r := range map[string]Request{
		// With no name, these actions would otherwise reach every package.
		"remove without a name":  {Action: "remove"},
		"disable without a name": {Action: "disable"},
		"enable without a name":  {Action: "enable"},
		"name and names":         {Action: "remove", Name: "a", Names: []string{"b"}},
		"another action":         {Action: "sync", Names: []string{"a", "b"}},
		"duplicate":              {Action: "remove", Names: []string{"a", "a"}},
		"invalid name":           {Action: "remove", Names: []string{"a", "../b"}},
		"unknown package":        {Action: "remove", Names: []string{"a", "missing"}},
	} {
		if _, err := s.Preview(context.Background(), r); err == nil {
			t.Errorf("%s: previewed", label)
		}
	}
}

// Two mods docked one after the other: b's layout copy was made from a's, and the window shows b's.
// Removing both together must remove both copies and leave the window on the original layout.
func TestNamesRemoveModsWhoseLayoutCopiesChain(t *testing.T) {
	s, herdr := newHerdrModsService(t, "a", "b")
	home, err := filepath.EvalSymlinks(herdr)
	if err != nil {
		t.Fatal(err)
	}
	layouts := map[string]string{"main": "", "main-skillshare": "main", "main-skillshare-skillshare": "main-skillshare"}
	receipts := map[string]herdrLayoutReceipt{}
	for name, from := range layouts {
		data := `{"id":"` + name + `"}`
		writePluginFile(t, home, "layouts/"+name+".json", data)
		if from != "" {
			receipts[name] = herdrLayoutReceipt{Home: home, From: from, Name: name, Digest: hash([]byte(data))}
		}
	}
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	for pkg, copyName := range map[string]string{"a": "main-skillshare", "b": "main-skillshare-skillshare"} {
		b := d.state.Bindings[pkg]["herdr-app"]
		b.HerdrLayouts = []herdrLayoutReceipt{receipts[copyName]}
		d.state.Bindings[pkg]["herdr-app"] = b
	}
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
	current := "main-skillshare-skillshare"
	s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		switch {
		case bin == "herdr-app" && args[0] == "status":
			return json.Marshal(map[string]string{"layout": current})
		case bin == "herdr-app" && args[0] == "do" && args[1] == "layout.switch":
			var req map[string]string
			if err := json.Unmarshal([]byte(args[2]), &req); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(home, "layouts", req["name"]+".json")); err != nil {
				t.Fatalf("switched to a removed layout %s", req["name"])
			}
			current = req["name"]
			return nil, nil
		}
		t.Fatalf("unexpected command %s %q", bin, args)
		return nil, nil
	}
	applyPluginRequest(t, s, Request{Action: "remove", Names: []string{"a", "b"}})
	for name := range receipts {
		if _, err := os.Stat(filepath.Join(home, "layouts", name+".json")); !os.IsNotExist(err) {
			t.Errorf("layout copy %s was left behind: %v", name, err)
		}
	}
	if current != "main" {
		t.Fatalf("window left on %s, want main", current)
	}
}
