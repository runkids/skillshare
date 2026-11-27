package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHerdrDockInstallRemoveReinstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated Herdr home uses POSIX fixtures")
	}
	home, source := t.TempDir(), t.TempDir()
	t.Setenv("HERDR_APP_HOME", home)
	writePluginFile(t, home, "layouts/main.json", herdrLayoutFixture)
	writePluginFile(t, home, "seen.json", `["demo"]`)
	writePluginFile(t, source, "mods/demo/main.js", "fixture")
	writePluginFile(t, source, "mods/demo/mod.json", `{"place":"right","size":340}`)
	mod := filepath.Join(home, "mods/demo/main.js")
	copy := filepath.Join(home, "layouts/main-skillshare.json")
	active := "main"
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		if bin != "herdr-app" {
			t.Fatalf("unexpected command %s %v", bin, args)
		}
		if args[0] == "status" {
			return json.Marshal(map[string]string{"layout": active})
		}
		var request map[string]string
		if args[0] != "do" || json.Unmarshal([]byte(args[2]), &request) != nil {
			t.Fatalf("unsupported command %v", args)
		}
		switch args[1] {
		case "layout.clone":
			if _, err := os.Stat(mod); !os.IsNotExist(err) {
				t.Fatalf("native mod appeared before switching off the original layout: %v", err)
			}
			if request["name"] != "main" || request["as"] != "main-skillshare" {
				t.Fatal(request)
			}
			writePluginFile(t, home, "layouts/main-skillshare.json", `{"cloned_from":{"name":"main"},`+herdrLayoutFixture[1:])
			active = request["as"]
		case "layout.switch":
			active = request["name"]
		default:
			t.Fatalf("unsupported command %v", args)
		}
		return nil, nil
	}}
	add := Request{Action: "add", Source: source, Plugin: "demo", Targets: []string{"herdr-app"}, HerdrPanel: true}
	for i := 0; i < 2; i++ {
		p, err := s.Preview(context.Background(), add)
		if err != nil || p.Blocked || p.HerdrPanel == nil || active != "main" {
			t.Fatalf("expected a reviewed layout without switching yet: %+v %v", p, err)
		}
		applyPluginRequest(t, s, add)
		data, err := os.ReadFile(copy)
		var tree struct {
			Children []struct {
				ID      string
				Content map[string]string
			}
		}
		if err != nil || json.Unmarshal(data, &tree) != nil || len(tree.Children) != 3 || tree.Children[2].Content["mod"] != "demo" || active != "main-skillshare" {
			t.Fatalf("dock was not placed on the right: %s %v", data, err)
		}
		if _, err := os.Stat(mod); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			// Previewing a panel for an already-installed mod still binds the
			// new placement hint, even when the native action is a no-op.
			active = "main"
			before, err := s.Preview(context.Background(), add)
			if err != nil || before.HerdrPanel == nil {
				t.Fatalf("preview existing mod's dock: %+v %v", before, err)
			}
			writePluginFile(t, source, "mods/demo/mod.json", `{"place":"left","size":340}`)
			after, err := s.Preview(context.Background(), add)
			if err != nil || after.Revision == before.Revision {
				t.Fatalf("a changed placement hint must invalidate the review: %+v %v", after, err)
			}
			writePluginFile(t, source, "mods/demo/mod.json", `{"place":"right","size":340}`)
			active = "main-skillshare"
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})
			applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			if active != "main" {
				t.Fatal("pausing did not hide the managed dock")
			}
			applyPluginRequest(t, s, Request{Action: "enable", Name: "demo"})
			applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			if active != "main-skillshare" {
				t.Fatal("resuming did not restore the managed dock")
			}
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
			if _, err := os.Stat(copy); !os.IsNotExist(err) || active != "main" {
				t.Fatalf("removal did not remove the dock copy: %v", err)
			}
		}
	}
	for path, expect := range map[string]string{"layouts/main.json": herdrLayoutFixture, "seen.json": `["demo"]`} {
		if data, err := os.ReadFile(filepath.Join(home, path)); err != nil || string(data) != expect {
			t.Fatalf("original changed: %s: %v", path, err)
		}
	}
}
