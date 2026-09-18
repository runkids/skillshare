package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativePackageLifecycle(t *testing.T) {
	for _, target := range []string{"copilot", "antigravity-cli"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			installed := false
			s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			s.Run = func(_ context.Context, _, bin string, args ...string) ([]byte, error) {
				command := strings.Join(args, " ")
				if command == "--version" {
					return []byte("test"), nil
				}
				if strings.Contains(command, "--help") {
					return []byte("Usage: plugin install uninstall list update --json"), nil
				}
				if strings.HasPrefix(command, "plugin list") {
					rows := []map[string]any{}
					if installed {
						rows = append(rows, map[string]any{"name": "demo", "enabled": false, "source": "installed"})
					}
					if bin == "agy" {
						return json.Marshal(map[string]any{"imports": rows})
					}
					return json.Marshal(rows)
				}
				if args[1] == "install" {
					installed = true
					return nil, nil
				}
				if args[1] == "uninstall" {
					installed = false
					return nil, nil
				}
				return nil, fmt.Errorf("unexpected command: %s %s", bin, command)
			}
			applyPluginRequest(t, s, Request{Action: "add", Source: fixture(t), Targets: []string{target}})
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})
			if !installed {
				t.Fatal("selection uninstalled immediately")
			}
			applyPluginRequest(t, s, Request{Action: "sync"})
			if installed {
				t.Fatal("sync failed to uninstall")
			}
			applyPluginRequest(t, s, Request{Action: "enable", Name: "demo"})
			applyPluginRequest(t, s, Request{Action: "sync"})
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
		})
	}
}

func TestFailedUpdateRestoresSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := fixture(t)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	installed, fail := false, false
	s.Run = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		if command == "--version" {
			return []byte("1"), nil
		}
		if strings.Contains(command, "--help") {
			return []byte("install"), nil
		}
		if command == "plugin list --json" {
			if installed {
				return []byte(`[{"name":"demo","enabled":true}]`), nil
			}
			return []byte(`[]`), nil
		}
		if fail {
			return nil, fmt.Errorf("native failure")
		}
		installed = true
		return nil, nil
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"copilot"}})
	cfg, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	binding := cfg.packages["demo"].Bindings["copilot"]
	writeFile(t, source, "skills/demo/SKILL.md", "new content")
	fail = true
	req := Request{Action: "update", Name: "demo"}
	plan, err := s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), req, plan.Revision); err == nil {
		t.Fatal("expected native failure")
	}
	digest, err := treeDigest(filepath.Join(s.snapshotPath(binding, "copilot"), "content"))
	if err != nil || digest != binding.Digest {
		t.Fatalf("old snapshot not restored: %s %v", digest, err)
	}
}
