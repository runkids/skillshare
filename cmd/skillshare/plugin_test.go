package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
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

func TestPluginCommandsCleanupListAndInspect(t *testing.T) {
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_CONFIG_DIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
		t.Setenv(key, "")
	}
	home, otherHome := t.TempDir(), t.TempDir()
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		for name, body := range map[string]string{
			"plugin.json": `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-command":"run"}}}}`,
			"run":         "#!/bin/sh\nprintf fixture\n",
		} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0755); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := &plugin.Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	other := &plugin.Service{ConfigPath: s.ConfigPath, StateDir: filepath.Join(otherHome, "state")}
	// Another machine replaces the shared declaration, retaining this machine's receipt.
	for _, step := range []struct {
		s    *plugin.Service
		home string
		r    plugin.Request
	}{{s, home, plugin.Request{Action: "add", Source: first}}, {other, otherHome, plugin.Request{Action: "remove", Name: "demo"}}, {other, otherHome, plugin.Request{Action: "add", Source: second}}} {
		t.Setenv("HOME", step.home)
		plan, err := step.s.Preview(context.Background(), step.r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := step.s.Apply(context.Background(), step.r, plan.Revision); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	for _, action := range []string{"list", "inspect"} {
		if action == "inspect" {
			if err := os.Remove(filepath.Join(home, ".local", "bin", "demo-command")); err != nil {
				t.Fatal(err)
			}
		}
		output := captureStdout(t, func() {
			if err := executePlugin(s, pluginOptions{request: plugin.Request{Action: action, Name: "demo"}, noTUI: true}); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(output, "commands · local cleanup required") || strings.Contains(output, "commands · pending") {
			t.Errorf("%s hid command cleanup: %s", action, output)
		}
	}
}

func TestPluginUpdateAllDryRunReportsFailures(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[jsonOutput], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			s := &plugin.Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			raw := "plugins:\n  version: 2\n  packages:\n    demo:\n      source: /missing-fixture-source\n      plugin: demo\n      bindings:\n        claude: {}\n"
			if err := os.WriteFile(s.ConfigPath, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			s.Run = func(context.Context, string, []string, string, ...string) ([]byte, error) {
				return nil, errors.New("fixture agent unavailable")
			}
			var runErr error
			output := captureStdout(t, func() {
				runErr = executePlugin(s, pluginOptions{request: plugin.Request{Action: "update", All: true}, dryRun: true, json: jsonOutput, noTUI: true})
			})
			if runErr == nil {
				t.Fatal("failed dry run returned success")
			}
			if jsonOutput {
				var plan plugin.Plan
				if err := json.Unmarshal([]byte(output), &plan); err != nil || len(plan.Changes) != 2 || plan.Changes[0].Target != "claude" || plan.Changes[0].Commands || !plan.Changes[0].Failed || plan.Changes[1].Target != "" || !plan.Changes[1].Commands || !plan.Changes[1].Failed || plan.Changes[1].MessageKey != "plugins.error.commandFailure" {
					t.Fatalf("bad JSON preview: %s %v", output, err)
				}
			} else if !strings.Contains(output, "· failed") || !strings.Contains(output, "demo") {
				t.Fatalf("missing failure: %s", output)
			}
		})
	}
}

func TestPluginUpdateAllDryRunReportsOMPFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"PI_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_PROFILE", "PI_PROFILE"} {
		t.Setenv(key, "")
	}
	source := filepath.Join(home, "source")
	if err := os.MkdirAll(filepath.Join(source, ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".claude-plugin", "plugin.json"), []byte(`{"name":"demo","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "skills", "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "demo", "SKILL.md"), []byte("---\nname: demo\ndescription: fixture\n---\nFixture"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &plugin.Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	raw := "plugins:\n  version: 2\n  packages:\n    demo:\n      source: " + source + "\n      plugin: demo\n      bindings:\n        omp: {}\n"
	if err := os.WriteFile(s.ConfigPath, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	s.Run = func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("fixture OMP unavailable")
	}
	var runErr error
	output := captureStdout(t, func() {
		runErr = executePlugin(s, pluginOptions{request: plugin.Request{Action: "update", All: true}, dryRun: true, json: true, noTUI: true})
	})
	var plan plugin.Plan
	if err := json.Unmarshal([]byte(output), &plan); runErr == nil || err != nil || len(plan.Changes) != 1 || plan.Changes[0].Target != "omp" || !plan.Changes[0].Failed || !strings.Contains(plan.Changes[0].Message, "OMP") {
		t.Fatalf("OMP dry-run failure missing: %s error=%v json=%v", output, runErr, err)
	}
}

func TestPluginWizardFirstCommandUpdate(t *testing.T) {
	for _, mode := range []string{"recorded", "deselected", "commandless", "command-bearing"} {
		t.Run(mode, func(t *testing.T) {
			deselected := mode == "deselected"
			native := mode == "commandless" || mode == "command-bearing"
			home, root := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_CONFIG_DIR"} {
				t.Setenv(key, "")
			}
			for name, body := range map[string]string{"plugin.json": `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","skills":"./skills"}`, "skills/demo/SKILL.md": "---\nname: demo\ndescription: fixture\n---\n", "run": "#!/bin/sh\nprintf first-command\n"} {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			s := &plugin.Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			r := plugin.Request{Action: "add", Source: root}
			if mode == "command-bearing" {
				if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","skills":"./skills","extensions":{"cc.runkids.skillshare":{"commands":{"demo-command":"run"}}}}`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if deselected || native {
				r.Targets = []string{"cursor"}
			}
			requests := []plugin.Request{r}
			if deselected {
				requests = append(requests, plugin.Request{Action: "disable", Name: "demo", Targets: []string{"cursor"}})
			}
			for _, r := range requests {
				p, err := s.Preview(context.Background(), r)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatal(err)
				}
			}
			before, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			if mode != "commandless" {
				if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","skills":"./skills","extensions":{"cc.runkids.skillshare":{"commands":{"demo-command":"run"}}}}`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			for path, body := range map[string]string{"skills/demo/SKILL.md": "---\nname: demo\ndescription: updated\n---\n", "run": "#!/bin/sh\nprintf updated-command\n"} {
				if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			inventory, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			pack := inventory.Packages["demo"]
			whole, offered := pluginWholePackageRequest(plugin.Request{Action: "update", Name: "demo", Targets: []string{"cursor"}}, pack)
			if !offered || len(whole.Targets) != 0 {
				t.Fatalf("wizard package choice: %+v %v", whole, offered)
			}
			if _, offered := pluginWholePackageRequest(plugin.Request{Action: "remove"}, pack); offered && pack.Commands == nil {
				t.Fatal("commandless removal offered command ownership")
			}
			p, err := s.Preview(context.Background(), whole)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, c := range p.Changes {
				if c.Commands && c.Action == "install" {
					found = true
				}
			}
			if !native && !found {
				t.Fatalf("Whole package omitted first command: %+v", p.Changes)
			}
			if _, err := s.Apply(context.Background(), whole, p.Revision); err != nil {
				t.Fatal(err)
			}
			after, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			if native {
				b := after.Packages["demo"].Bindings["cursor"]
				if b.Digest == "" || b.Digest == before.Packages["demo"].Bindings["cursor"].Digest {
					t.Error("Whole package left native Cursor digest unchanged")
				}
				content, err := os.ReadFile(filepath.Join(home, ".cursor/plugins/local", b.ID, "skills/demo/SKILL.md"))
				if err != nil || !strings.Contains(string(content), "description: updated") {
					t.Errorf("Whole package left native content stale: %s %v", content, err)
				}
			}
			if mode != "commandless" {
				output, err := exec.Command(filepath.Join(after.Packages["demo"].Commands.Bin, "demo-command")).Output()
				if err != nil || string(output) != "updated-command" {
					t.Errorf("Whole package command output: %q %v", output, err)
				}
			}
			if deselected && after.Packages["demo"].Bindings["cursor"].Selected() {
				t.Error("Whole package enabled deselected Cursor")
			}
		})
	}
}
