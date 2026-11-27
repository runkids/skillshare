package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const herdrLayoutFixture = `{"id":"root","type":"row","children":[{"id":"sidebar","type":"block","content":{"mod":"spaces"}},{"id":"work","type":"column","children":[{"id":"tabs","type":"block","size":{"px":42},"window_drag":true,"content":{"mod":"tabs"}},{"id":"work-body","type":"row","children":[{"id":"main","type":"block","content":{"tab":"active"}},{"id":"files","type":"block","content":{"mod":"files"}}]},{"id":"status","type":"block","content":{"mod":"custom-meter"}}]}]}`

func TestHerdrPanelCopiesReviewedLayout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file URL fixture uses a POSIX path")
	}
	ctx := context.Background()
	t.Chdir(t.TempDir())
	home := t.TempDir()
	t.Setenv("HERDR_APP_HOME", home)
	writePluginFile(t, home, "layouts/main.json", herdrLayoutFixture)
	current := "main"
	s := &Service{Run: func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		if bin != "herdr-app" {
			t.Fatalf("unexpected command %s %q", bin, args)
		}
		switch args[0] {
		case "status":
			return json.Marshal(map[string]string{"layout": current})
		case "do":
			if args[1] == "layout.switch" {
				var req map[string]string
				if err := json.Unmarshal([]byte(args[2]), &req); err != nil || req["name"] != "main" && req["name"] != "main-skillshare" {
					t.Fatalf("unexpected layout switch %q", args)
				}
				current = req["name"]
				return nil, nil
			}
			if args[1] != "layout.clone" {
				t.Fatalf("unexpected Herdr command %q", args)
			}
			var req map[string]string
			if err := json.Unmarshal([]byte(args[2]), &req); err != nil || req["name"] != "main" || req["as"] != "main-skillshare" {
				t.Fatalf("wrong clone %v %v", req, err)
			}
			writePluginFile(t, home, "layouts/main-skillshare.json", `{"cloned_from":{"name":"main"},`+herdrLayoutFixture[1:])
			writePluginFile(t, home, "layouts/.state/main-skillshare.json", `{"theme":"graphite","file":{"id":"old"}}`)
			current = "main-skillshare"
			return nil, nil
		default:
			t.Fatalf("unexpected Herdr command %q", args)
			return nil, nil
		}
	}}
	repo := t.TempDir()
	writePluginFile(t, repo, "herdr/skillshare-mod/mods/skillshare/main.js", "fixture")
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "mod"}} {
		if _, err := runCommand(ctx, repo, nil, "git", args...); err != nil {
			t.Fatal(err)
		}
	}
	writePluginFile(t, home, "gitconfig", "[url \"file://"+repo+"\"]\n  insteadOf = https://github.com/runkids/skillshare.git\n")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	s.ConfigPath, s.StateDir = filepath.Join(home, "config.yaml"), filepath.Join(home, "state")
	request := Request{Action: "add", Source: "https://github.com/runkids/skillshare/tree/main/herdr/skillshare-mod", Plugin: "skillshare", Targets: []string{"herdr-app"}, HerdrPanel: true}
	p, err := s.Preview(ctx, request)
	if err != nil || p.HerdrPanel == nil || p.HerdrPanel.Name != "main-skillshare" || current != "main" {
		t.Fatalf("preview mutated layout or picked wrong name: %+v %v", p, err)
	}
	mod := filepath.Join(home, "mods/skillshare/main.js")
	writePluginFile(t, home, "layouts/main.json", herdrLayoutFixture+"\n")
	if _, err := s.Apply(ctx, request, p.Revision); err == nil || current != "main" {
		t.Fatalf("applied stale approval: %v", err)
	}
	if _, err := os.Stat(mod); !os.IsNotExist(err) {
		t.Fatalf("installed before layout approval: %v", err)
	}
	writePluginFile(t, home, "layouts/main.json", herdrLayoutFixture)
	if result, err := s.Apply(ctx, request, p.Revision); err != nil || len(result.Results) != 2 || result.Results[1].MessageKey != "plugins.outcome.herdrPanelShown" {
		t.Fatalf("installation and placement failed: %+v %v", result, err)
	}
	if got, err := os.ReadFile(mod); err != nil || string(got) != "fixture" {
		t.Fatalf("mod not installed: %q %v", got, err)
	}
	old, _ := os.ReadFile(filepath.Join(home, "layouts/main.json"))
	if string(old) != herdrLayoutFixture || current != "main-skillshare" {
		t.Fatalf("original layout changed or copy is not active")
	}
	result, err := os.ReadFile(filepath.Join(home, "layouts/main-skillshare.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"skillshare-toggle", "skillshare-panel", "custom-meter", "spaces", "files", "window_drag"} {
		if !bytes.Contains(result, []byte(`"`+value+`"`)) {
			t.Errorf("layout missing %s: %s", value, result)
		}
	}
	if next, err := s.previewHerdrPanel(ctx); err != nil || next != nil {
		t.Fatalf("reattach would duplicate nodes: %+v %v", next, err)
	}

	copyPath := filepath.Join(home, "layouts/main-skillshare.json")
	applyPluginRequest(t, s, Request{Action: "disable", Name: "skillshare", Targets: []string{"herdr-app"}})
	applyPluginRequest(t, s, Request{Action: "sync", Name: "skillshare"})
	if _, err := os.Stat(mod); !os.IsNotExist(err) {
		t.Fatalf("temporary sync did not uninstall mod: %v", err)
	}
	if data, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(data, result) {
		t.Fatalf("temporary sync changed the layout copy: %v", err)
	}
	if current != "main" {
		t.Fatalf("unloaded mod still has a visible layout reference: %s", current)
	}
	// An earlier version can already have uninstalled the mod while leaving its layout active.
	current = "main-skillshare"
	applyPluginRequest(t, s, Request{Action: "sync", Name: "skillshare"})
	if current != "main" {
		t.Fatalf("sync did not clear the previously unloaded panel: %s", current)
	}
	applyPluginRequest(t, s, Request{Action: "enable", Name: "skillshare", Targets: []string{"herdr-app"}})
	applyPluginRequest(t, s, Request{Action: "sync", Name: "skillshare"})
	if _, err := os.Stat(mod); err != nil || current != "main-skillshare" {
		t.Fatalf("sync did not restore the mod and its saved layout: %s %v", current, err)
	}

	remove := Request{Action: "remove", Name: "skillshare"}
	plan, err := s.Preview(ctx, remove)
	if err != nil || plan.Blocked {
		t.Fatalf("permanent removal blocked: %+v %v", plan, err)
	}
	writePluginFile(t, home, "layouts/main-skillshare.json", string(result)+"\n")
	if _, err := s.Apply(ctx, remove, plan.Revision); err == nil || current != "main-skillshare" {
		t.Fatalf("permanent removal applied a stale layout approval: %v", err)
	}
	if _, err := os.Stat(copyPath); err != nil {
		t.Fatalf("edited copy was removed: %v", err)
	}
	writePluginFile(t, home, "layouts/main-skillshare.json", string(result))
	writePluginFile(t, home, "layouts/.state/main-skillshare.json", `{"theme":"paper"}`)
	keep, err := s.Preview(ctx, remove)
	if err != nil || keep.Blocked || keep.Changes[0].MessageKey != "plugins.note.herdrLayoutKeep" {
		t.Fatalf("layout click edits were not protected: %+v %v", keep, err)
	}
	// Herdr saves a new "file" baseline without changing the user's existing clicks.
	writePluginFile(t, home, "layouts/.state/main-skillshare.json", `{"file":{"id":"new"},"theme":"graphite"}`)
	applyPluginRequest(t, s, remove)
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) || current != "main" {
		t.Fatalf("permanent removal did not delete the copy and return to original: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "layouts/.state/main-skillshare.json")); !os.IsNotExist(err) {
		t.Fatalf("permanent removal left state for the removed copy: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(home, "layouts/main.json")); err != nil || string(data) != herdrLayoutFixture {
		t.Fatalf("permanent removal modified original layout: %v", err)
	}
}
