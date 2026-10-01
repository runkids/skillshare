package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The native boundary replaces caches and sets enabled=true even on a disabled
// install. Exercise the service's filesystem transaction around those effects.
func TestCodexUpdateTransaction(t *testing.T) {
	for _, mode := range []string{"enabled", "disabled", "wrong-version", "partial-failure", "policy", "comment-conflict", "cancelled", "same-version-race", "sibling-conflict", "preflight-conflict"} {
		t.Run(mode, func(t *testing.T) {
			s, source, home := codexUpdateFixture(t, mode == "enabled")
			configPath := filepath.Join(home, "config.toml")
			before, _ := os.ReadFile(configPath)
			writeFile(t, home, "plugins/cache/team/other/4.0.0/keep", "unrelated bytes")
			if err := os.Chmod(filepath.Join(home, "plugins/cache/team/other/4.0.0"), 0700); err != nil {
				t.Fatal(err)
			}
			original := s.Run
			added := false
			inventoryCalls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				if strings.Join(args, " ") == "plugin list --json" {
					inventoryCalls++
					if mode == "preflight-conflict" && inventoryCalls == 3 {
						writeFile(t, home, "plugins/cache/team/demo/1.0.0/notes", "external selected edit")
					}
				}
				if added && mode == "sibling-conflict" && strings.Join(args, " ") == "plugin list --json" {
					writeFile(t, home, "plugins/cache/team/other/4.0.0/keep", "external sibling edit")
				}
				if len(args) > 2 && args[0] == "plugin" && args[1] == "add" && args[2] == "demo@team" {
					if mode == "policy" {
						return nil, fmt.Errorf("native authentication required")
					}
					var staged string
					for _, arg := range args {
						if strings.HasPrefix(arg, `marketplaces."team".source=`) {
							if err := json.Unmarshal([]byte(strings.TrimPrefix(arg, `marketplaces."team".source=`)), &staged); err != nil {
								return nil, err
							}
						}
					}
					if staged == "" || staged == source {
						return nil, fmt.Errorf("missing private reviewed source")
					}
					if mode == "same-version-race" {
						writeFile(t, source, "skills/demo/SKILL.md", "changed during install")
					}
					version := "1.0.1"
					if mode == "wrong-version" {
						version = "9.0.0"
					}
					if err := os.RemoveAll(filepath.Join(home, "plugins/cache/team/demo")); err != nil {
						return nil, err
					}
					writeFile(t, home, "plugins/cache/team/demo/"+version+"/.codex-plugin/plugin.json", fmt.Sprintf(`{"name":"demo","version":%q}`, version))
					writeFile(t, home, "plugins/cache/team/demo/"+version+"/skills/demo/SKILL.md", "new content")
					next := strings.Replace(string(before), "enabled = false", "enabled = true", 1)
					if mode == "comment-conflict" {
						next += "\n# external comment\n"
					}
					writeFile(t, home, "config.toml", next)
					added = true
					if mode == "cancelled" {
						cancel()
						return nil, context.Canceled
					}
					if mode == "partial-failure" {
						return nil, fmt.Errorf("native install failed after replacement")
					}
					return []byte(`{"installed":true,"pluginId":"demo@team"}`), nil
				}
				return original(ctx, dir, env, bin, args...)
			}
			r := Request{Action: "update", Name: "demo", Targets: []string{"codex"}}
			p, err := s.Preview(ctx, r)
			if err != nil || p.Blocked {
				t.Fatalf("preview: %+v %v", p, err)
			}
			_, err = s.Apply(ctx, r, p.Revision)
			wantSuccess := mode == "enabled" || mode == "disabled"
			if wantSuccess && err != nil {
				t.Fatal(err)
			}
			if !wantSuccess && err == nil {
				t.Fatal("unverified native update recorded success")
			}
			after, _ := os.ReadFile(configPath)
			if mode == "comment-conflict" {
				if !strings.Contains(string(after), "external comment") || !strings.Contains(err.Error(), "recovery") {
					t.Fatalf("concurrent edit overwritten or recovery missing: %s %v", after, err)
				}
			} else if string(after) != string(before) {
				t.Fatalf("configuration not preserved:\n%s", after)
			}
			if wantSuccess {
				if _, err := os.Stat(filepath.Join(home, "plugins/cache/team/demo/1.0.1")); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err := os.ReadFile(filepath.Join(home, "plugins/cache/team/demo/1.0.0/skills/demo/SKILL.md"))
				if err != nil || string(data) != "old content" {
					t.Fatalf("working install lost: %q %v", data, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(home, "plugins/cache/team/other/4.0.0/keep"))
			wantSibling := "unrelated bytes"
			if mode == "sibling-conflict" {
				wantSibling = "external sibling edit"
			}
			if err != nil || string(data) != wantSibling {
				t.Fatalf("sibling changed: %q %v", data, err)
			}
			info, err := os.Stat(filepath.Join(home, "plugins/cache/team/other/4.0.0"))
			if err != nil || info.Mode().Perm() != 0700 {
				t.Fatalf("sibling mode changed: %v %v", info, err)
			}
			d, err := s.load()
			if err != nil {
				t.Fatal(err)
			}
			binding := d.packages["demo"].Bindings["codex"]
			if (binding.Pending == "") != wantSuccess {
				t.Fatalf("pending state contradicts verification: %+v", binding)
			}
		})
	}
}
