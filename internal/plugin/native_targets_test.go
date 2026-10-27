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
			s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
				command := strings.Join(args, " ")
				if command == "--version" {
					return []byte("test"), nil
				}
				if strings.Contains(command, "--help") {
					return []byte("Usage: plugin install uninstall list update --json"), nil
				}
				if command == "plugin list --json" {
					rows := []map[string]any{}
					if installed {
						rows = append(rows, map[string]any{"name": "demo", "enabled": false, "source": "installed"})
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
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
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

func TestParseCommandInventoryAntigravityCLI(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantError bool
		check     func(t *testing.T, items []Installed)
	}{
		{
			name:    "empty json array",
			input:   "[]\n",
			wantLen: 0,
		},
		{
			name:    "text no plugins installed",
			input:   "No plugins installed.\n",
			wantLen: 0,
		},
		{
			name:    "text no imported plugins",
			input:   "No imported plugins.\n",
			wantLen: 0,
		},
		{
			name:    "empty string",
			input:   "  \n",
			wantLen: 0,
		},
		{
			name: "native json array with plugin",
			input: `[
  {
    "name": "beads",
    "enabled": true,
    "version": "1.3.2",
    "source": "claude-code"
  }
]`,
			wantLen: 1,
			check: func(t *testing.T, items []Installed) {
				item := items[0]
				if item.ID != "beads" || item.Name != "beads" || item.Version != "1.3.2" || !item.Enabled || !item.EnabledKnown {
					t.Fatalf("unexpected parsed item: %+v", item)
				}
			},
		},
		{
			name:    "legacy envelope imports",
			input:   `{"imports": [{"name": "demo", "enabled": false, "version": "1.0.0"}]}`,
			wantLen: 1,
			check: func(t *testing.T, items []Installed) {
				item := items[0]
				if item.ID != "demo" || item.Name != "demo" || item.Version != "1.0.0" || item.Enabled || !item.EnabledKnown {
					t.Fatalf("unexpected parsed item: %+v", item)
				}
			},
		},
		{
			name:      "invalid json",
			input:     `not json`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := parseCommandInventory("antigravity-cli", []byte(tt.input))
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(items) != tt.wantLen {
				t.Fatalf("got %d items, want %d", len(items), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, items)
			}
		})
	}
}

func TestAntigravityCLIUpdateRespectsEnablement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := fixture(t)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	installed, enabled := false, true
	var installs int
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		if command == "--version" {
			return []byte("1.0.0"), nil
		}
		if strings.Contains(command, "--help") {
			return []byte("Usage: agy plugin install uninstall list"), nil
		}
		if command == "plugin list --json" {
			if !installed {
				return []byte(`[]`), nil
			}
			return json.Marshal([]map[string]any{{"name": "demo", "enabled": enabled, "version": "1.0.0"}})
		}
		if len(args) >= 2 && args[1] == "install" {
			installed = true
			installs++
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command: %s", command)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"antigravity-cli"}})
	writeFile(t, source, "skills/demo/SKILL.md", "updated v2")

	// When disabled or absent in Antigravity CLI, update must skip to preserve native enablement.
	for _, tc := range []struct {
		installed bool
		enabled   bool
	}{
		{installed: true, enabled: false},
		{installed: false, enabled: true},
	} {
		installed, enabled = tc.installed, tc.enabled
		plan, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo"})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Changes) != 1 || plan.Changes[0].Action != "skip" || plan.Changes[0].MessageKey != "plugins.skip.keepEnablement" {
			t.Fatalf("expected keepEnablement skip for agy plugin (installed=%v, enabled=%v), got %+v", tc.installed, tc.enabled, plan.Changes)
		}
	}

	// When installed and enabled in Antigravity CLI, update must proceed and reinstall the updated snapshot.
	installed, enabled = true, true
	before := installs
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	if installs != before+1 {
		t.Fatalf("expected agy plugin install on enabled update, installs=%d (before=%d)", installs, before)
	}
}
