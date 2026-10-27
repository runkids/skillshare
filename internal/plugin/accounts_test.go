package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// accountPluginService has a second Claude account: the target claude-work is Claude with
// another config directory, so its plugins are installed through that directory.
func accountPluginService(t *testing.T, calls *[]string) *Service {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &Service{
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
		Accounts:   map[string]Account{"claude-work": {Agent: "claude", Dir: filepath.Join(home, ".claude-work")}},
	}
	s.Run = func(_ context.Context, _ string, env []string, bin string, args ...string) ([]byte, error) {
		*calls = append(*calls, bin+" "+strings.Join(env, " "))
		if args[0] == "--version" {
			return []byte("2.1.276"), nil
		}
		if slices.Contains(args, "--help") {
			return []byte("install uninstall update --scope --json"), nil
		}
		return []byte(`[]`), nil
	}
	return s
}

func TestAccountRunsTheAgentCLIPointedAtItsDirectory(t *testing.T) {
	var calls []string
	s := accountPluginService(t, &calls)
	if h := s.host(context.Background(), "claude-work"); h.Target != "claude-work" || h.Status != HostReady {
		t.Fatalf("host = %+v", h)
	}
	want := "claude CLAUDE_CONFIG_DIR=" + s.Accounts["claude-work"].Dir
	if !slices.Contains(calls, want) {
		t.Fatalf("no call was %q: %v", want, calls)
	}
	calls = nil
	s.host(context.Background(), "claude")
	if slices.ContainsFunc(calls, func(c string) bool { return strings.Contains(c, "CLAUDE_CONFIG_DIR") }) {
		t.Fatalf("the Agent itself was given a config directory: %v", calls)
	}
}

func TestAccountTakesTheAgentsPluginOperations(t *testing.T) {
	var calls []string
	s := accountPluginService(t, &calls)
	definitions := s.TargetDefinitions()
	i := slices.IndexFunc(definitions, func(d TargetDefinition) bool { return d.Target == "claude-work" })
	if i < 0 || definitions[i].Label != "claude-work" || definitions[i].Project || !slices.Contains(definitions[i].Operations, "add") {
		t.Fatalf("definitions = %+v", definitions)
	}
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: fixture(t), Plugin: "demo", Targets: []string{"claude-work"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Target != "claude-work" || p.Changes[0].Action != "install" {
		t.Fatalf("changes = %+v", p.Changes)
	}
}

func TestAccountIsOfferedWhereverItsAgentIs(t *testing.T) {
	var calls []string
	s := accountPluginService(t, &calls)
	d, err := s.Discover(context.Background(), fixture(t), "", "")
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if !slices.Contains(c.Targets, "claude-work") || c.TargetInfo["claude-work"].Manifest != c.TargetInfo["claude"].Manifest {
		t.Fatalf("candidate = %+v", c)
	}
}

func TestAccountBindingNeedsTheAccountToBeDeclared(t *testing.T) {
	raw := []byte("plugins:\n  packages:\n    demo:\n      bindings:\n        claude-work:\n          id: demo@market\n")
	if err := Validate(raw, nil); err == nil {
		t.Fatal("a name that is no target was accepted")
	}
	if err := Validate(raw, map[string]string{"claude-work": "claude"}); err != nil {
		t.Fatalf("the account's binding was refused: %v", err)
	}
}

// An account can run a compatible CLI instead of its Agent's, such as omo for Pi.
func TestAccountRunsItsCLI(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "pi", &calls)
	a := s.Accounts["pi-work"]
	a.CLI = "omo"
	s.Accounts["pi-work"] = a
	s.host(context.Background(), "pi-work")
	if !slices.ContainsFunc(calls, func(c string) bool { return strings.HasPrefix(c, "omo ") }) {
		t.Fatalf("omo was not run: %v", calls)
	}
	calls = nil
	s.host(context.Background(), "pi")
	if slices.ContainsFunc(calls, func(c string) bool { return strings.HasPrefix(c, "omo ") }) {
		t.Fatalf("the Agent itself ran the account's CLI: %v", calls)
	}
}

// Pi forks read their own variable before PI_CODING_AGENT_DIR (omo: OMO_, then SENPI_), so
// one exported in the user's shell would send the account's plugins to another directory.
func TestPiAccountPointsEveryPiVariableAtItsDirectory(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "pi", &calls)
	dir := s.Accounts["pi-work"].Dir
	s.host(context.Background(), "pi-work")
	want := "pi PI_CODING_AGENT_DIR=" + dir + " SENPI_CODING_AGENT_DIR=" + dir + " OMO_CODING_AGENT_DIR=" + dir
	if !slices.Contains(calls, want) {
		t.Fatalf("no call was %q: %v", want, calls)
	}
}

// A path that does not exist is a missing CLI, the same as a name not on PATH.
func TestMissingCLIPathIsReportedAsMissing(t *testing.T) {
	_, err := runCommand(context.Background(), t.TempDir(), nil, filepath.Join(t.TempDir(), "omo"), "--version")
	if !errors.Is(err, ErrCLIMissing) {
		t.Fatalf("err = %v", err)
	}
}

// A working directory that is gone fails with the same error as a missing executable.
func TestMissingDirectoryIsNotReportedAsMissingCLI(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	_, err = runCommand(context.Background(), filepath.Join(t.TempDir(), "gone"), nil, sh, "-c", "true")
	if err == nil || errors.Is(err, ErrCLIMissing) {
		t.Fatalf("err = %v", err)
	}
}

// agentAccountPluginService has one account of agent, named <agent>-work.
func agentAccountPluginService(t *testing.T, agent string, calls *[]string) *Service {
	t.Helper()
	s := accountPluginService(t, calls)
	home := filepath.Dir(s.ConfigPath)
	s.Accounts = map[string]Account{agent + "-work": {Agent: agent, Dir: filepath.Join(home, "."+agent+"-work")}}
	return s
}

func TestCodexAccountRunsItsCLIWithCodexHome(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "codex", &calls)
	s.host(context.Background(), "codex-work")
	want := "codex CODEX_HOME=" + s.Accounts["codex-work"].Dir
	if !slices.Contains(calls, want) {
		t.Fatalf("no call was %q: %v", want, calls)
	}
}

func TestOpenCodeAccountRunsItsCLIWithOpenCodeConfigDir(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "opencode", &calls)
	s.host(context.Background(), "opencode-work")
	want := "opencode OPENCODE_CONFIG_DIR=" + s.Accounts["opencode-work"].Dir + " OPENCODE_CONFIG="
	if !slices.Contains(calls, want) {
		t.Fatalf("no call was %q: %v", want, calls)
	}
}

func TestOpenCodeAccountRefusesOpenCodeConfigOverride(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "opencode", &calls)
	t.Setenv("OPENCODE_CONFIG", filepath.Join(t.TempDir(), "other.json"))
	_, err := s.openCodeConfigPath("opencode-work")
	if err == nil || !strings.Contains(err.Error(), "unset OPENCODE_CONFIG before account plugin sync") {
		t.Fatalf("expected error rejecting OPENCODE_CONFIG, got %v", err)
	}
}

func TestOpenCodeAccountUpdateProbeUsesConfiguredCLIAndEnv(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "opencode", &calls)
	a := s.Accounts["opencode-work"]
	a.CLI = "/custom/bin/opencode-v2"
	s.Accounts["opencode-work"] = a

	cfg := []byte("plugins:\n  packages:\n    demo:\n      bindings:\n        opencode-work:\n          id: demo\n")
	if err := os.WriteFile(s.ConfigPath, cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	seedLocalBinding(t, s, "demo", "opencode-work", Binding{ID: "demo"})
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"opencode-work"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "update" {
		t.Fatalf("unexpected changes: %+v", p.Changes)
	}

	wantProbePrefix := "/custom/bin/opencode-v2 OPENCODE_CONFIG_DIR=" + a.Dir + " OPENCODE_CONFIG="
	found := false
	for _, call := range calls {
		if strings.HasPrefix(call, wantProbePrefix) {
			found = true
		}
		if strings.HasPrefix(call, "opencode ") {
			t.Fatalf("probe called default opencode instead of account CLI: %s", call)
		}
	}
	if !found {
		t.Fatalf("expected probe call starting with %q, got: %v", wantProbePrefix, calls)
	}
}

// OpenCode's packages are a file, so an account's inventory must be read
// from the account's own directory rather than default config or OPENCODE_CONFIG_DIR.
func TestOpenCodeAccountInventoryComesFromItsDirectory(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "opencode", &calls)
	dir := s.Accounts["opencode-work"].Dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"plugins":["demo-pkg"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h := s.host(context.Background(), "opencode-work")
	if h.Status != HostReady {
		t.Fatalf("host = %+v", h)
	}
	if len(h.Installed) != 1 || h.Installed[0].ID != "demo-pkg" {
		t.Fatalf("installed = %+v", h.Installed)
	}
}

// Pi's packages are a file, not a list command, so an account's inventory must be read
// from the account's own directory rather than from PI_CODING_AGENT_DIR.
func TestPiAccountInventoryComesFromItsDirectory(t *testing.T) {
	var calls []string
	s := agentAccountPluginService(t, "pi", &calls)
	dir := s.Accounts["pi-work"].Dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"packages":["npm:demo"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h := s.host(context.Background(), "pi-work")
	if h.Status != HostReady {
		t.Fatalf("host = %+v", h)
	}
	if len(h.Installed) != 1 || h.Installed[0].ID != "npm:demo" {
		t.Fatalf("installed = %+v", h.Installed)
	}
}

// An account's bindings are validated by the Agent that installs them: Pi identifiers are
// not Claude's name@marketplace.
func TestAccountBindingIsValidatedByItsAgent(t *testing.T) {
	raw := []byte("plugins:\n  packages:\n    demo:\n      bindings:\n        pi-work:\n          id: npm:demo\n")
	if err := Validate(raw, map[string]string{"pi-work": "pi"}); err != nil {
		t.Fatalf("a Pi identifier was refused for a Pi account: %v", err)
	}
	if err := Validate(raw, map[string]string{"pi-work": "claude"}); err == nil {
		t.Fatal("a Pi identifier was accepted for a Claude account")
	}
}
