package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/plugin"
)

func TestPluginOptions(t *testing.T) {
	o, err := parsePluginOptions([]string{"add", "owner/repo", "--plugin", "demo", "--target", "claude", "--target", "codex", "--json"})
	if err != nil || o.request.Source != "owner/repo" || len(o.request.Targets) != 2 || !o.json {
		t.Fatalf("%+v %v", o, err)
	}
	for _, args := range [][]string{{"unknown"}, {"add", "source", "--target"}, {"remove", "a", "b"}, {"list", "--oops"}, {"update", "demo", "--all"}, {"update", "--name", "demo", "-a"}, {"update", "--all", "--source-ref", "main"}, {"sync", "--all"}} {
		if _, err := parsePluginOptions(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, flag := range []string{"--all", "-a"} {
		u, err := parsePluginOptions([]string{"update", flag, "--target", "claude"})
		if err != nil || !u.request.All || u.request.Name != "" || len(u.request.Targets) != 1 {
			t.Fatalf("update %s: %+v %v", flag, u, err)
		}
	}
}

func TestPluginCompletionsIncludeAllFlag(t *testing.T) {
	want := map[string]string{
		"bash":       "plugin)     COMPREPLY=($(compgen -W \"add discover import unimport list inspect sync check update enable disable remove --target --from --plugin --name --source-ref --entry --revision --all -a",
		"zsh":        "'--all[Update all managed plugins (update only)]' \\\n                        '-a[Update all managed plugins (update only)]'",
		"fish":       "complete -c skillshare -n '__fish_skillshare_using_command plugin' -l all -s a",
		"powershell": "'plugin' = '--target', '--from', '--plugin', '--name', '--source-ref', '--entry', '--revision', '--all', '-a'",
		"nushell":    "--revision: string\n    --all(-a)",
	}
	for shell, snippet := range want {
		if !strings.Contains(shells[shell].script, snippet) {
			t.Fatalf("%s completion does not advertise plugin --all/-a", shell)
		}
	}
}

func TestPluginAntigravityAlias(t *testing.T) {
	o, err := parsePluginOptions([]string{"add", "./demo", "--target", "agy", "--no-tui"})
	if err != nil || o.request.Targets[0] != "antigravity" {
		t.Fatalf("alias: %+v %v", o, err)
	}
}

func TestPluginWizardRefreshesEarlierAddDiscovery(t *testing.T) {
	home, source := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi/agent"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	if err := os.MkdirAll(filepath.Join(source, "extensions"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "extensions/demo.ts"), []byte("export default () => {}"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := func(version string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"name":"demo","version":"`+version+`","keywords":["pi-package"]}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	manifest("1.0.0")
	s := &plugin.Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("native CLI unavailable")
	}
	// This is the earlier manager action's discovery scope. A wizard must start
	// its own action, even when its source was already observed in this session.
	_ = s.WithDiscoveryCache(func() error {
		if _, err := s.Preview(context.Background(), plugin.Request{Action: "add", Source: source}); err != nil {
			t.Fatal(err)
		}
		manifest("2.0.0")
		request := plugin.Request{Action: "add", Source: source, Plugin: "demo", Targets: []string{"pi"}}
		output := captureStdout(t, func() {
			err := pluginWizard(s, pluginOptions{request: request, noTUI: true})
			if err == nil || !strings.Contains(err.Error(), "blocked") {
				t.Fatalf("expected unavailable native CLI to stop before confirmation: %v", err)
			}
		})
		fresh, err := s.Preview(context.Background(), request)
		if err != nil || len(fresh.Changes) != 1 || fresh.Changes[0].Binding.Version != "2.0.0" || !strings.Contains(output, fresh.Revision) {
			t.Fatalf("wizard reused the earlier add discovery: %+v %v output=%s", fresh, err, output)
		}
		return nil
	})
	for _, path := range []string{s.ConfigPath, s.StateDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("read-only blocked action wrote %s: %v", path, err)
		}
	}
}

func TestPluginUnimportOptions(t *testing.T) {
	o, err := parsePluginOptions([]string{"unimport", "review", "--target", "claude", "--dry-run", "--json"})
	if err != nil || o.request.Action != "unimport" || o.request.Name != "review" || len(o.request.Targets) != 1 || o.request.Targets[0] != "claude" || !o.dryRun || !o.json {
		t.Fatalf("unimport options: %+v %v", o, err)
	}
	for name, shell := range shells {
		if !strings.Contains(shell.script, "unimport") {
			t.Errorf("%s omits unimport", name)
		}
	}
}

func TestPluginRetainedImportListAndDoctor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &plugin.Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "plugin list --json":
			return []byte(`[{"id":"demo@team","name":"demo","scope":"user","installed":true}]`), nil
		case "plugin marketplace list --json":
			return []byte(`[]`), nil
		default:
			return []byte("--json --scope"), nil
		}
	}
	raw := "plugins:\n  version: 2\n  packages:\n    review:\n      source: https://fixture.invalid/demo.git\n      plugin: demo\n      bindings:\n        claude: {}\n"
	if err := os.WriteFile(s.ConfigPath, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	req := plugin.Request{Action: "import", Name: "review", From: "claude", Plugin: "demo@team"}
	p, err := s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), req, p.Revision); err != nil {
		t.Fatal(err)
	}
	raw = strings.Replace(raw, "      plugin: demo", "      source_ref: next\n      plugin: demo", 1)
	if err := os.WriteFile(s.ConfigPath, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := executePlugin(s, pluginOptions{request: plugin.Request{Action: "list"}, noTUI: true}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "local import retained") {
		t.Errorf("list hid retained import: %s", output)
	}
	result := &doctorResult{}
	captureStdout(t, func() { checkPluginPackages(s, result) })
	if len(result.checks) != 1 || result.checks[0].Status != "warning" {
		t.Fatalf("doctor claimed in sync: %+v", result.checks)
	}
}
