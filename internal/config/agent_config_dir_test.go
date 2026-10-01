package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// loadWithTargets writes a global config whose targets section is body, with $ROOT
// replaced by a temp dir, and loads it.
func loadWithTargets(t *testing.T, body string) (*Config, string, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	data := "source: $ROOT/skills\ntargets:\n" + body
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(data, "$ROOT", root)), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	return cfg, root, err
}

// A target can be another config directory of a built-in Agent, such as a second Claude
// account: its paths are the Agent's own, moved into that directory.
func TestAgentConfigDir_DerivesThePaths(t *testing.T) {
	cfg, root, err := loadWithTargets(t, "  claude-work:\n    agent: claude\n    config_dir: $ROOT/.claude-work\n")
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["claude-work"]
	if got := target.SkillsConfig().Path; got != filepath.Join(root, ".claude-work", "skills") {
		t.Errorf("skills path %q", got)
	}
	if got := target.AgentsConfig().Path; got != filepath.Join(root, ".claude-work", "agents") {
		t.Errorf("agents path %q", got)
	}
}

func TestAgentConfigDir_AnExplicitPathWins(t *testing.T) {
	cfg, root, err := loadWithTargets(t, "  claude-work:\n    agent: claude\n    config_dir: $ROOT/.claude-work\n    skills:\n      path: $ROOT/elsewhere\n")
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["claude-work"]
	if got := target.SkillsConfig().Path; got != filepath.Join(root, "elsewhere") {
		t.Errorf("skills path %q", got)
	}
}

// Derived paths follow config_dir, so they must not be written back as explicit ones.
func TestAgentConfigDir_SaveKeepsThePathsDerived(t *testing.T) {
	cfg, _, err := loadWithTargets(t, "  claude-work:\n    agent: claude\n    config_dir: $ROOT/.claude-work\n    skills:\n      mode: copy\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ConfigPath())
	if strings.Contains(string(data), "path:") || !strings.Contains(string(data), "mode: copy") {
		t.Fatalf("saved config:\n%s", data)
	}
	if target := cfg.Targets["claude-work"]; target.SkillsConfig().Path == "" {
		t.Error("saving cleared the path of the loaded config")
	}
}

func TestAgentConfigDir_Rejected(t *testing.T) {
	home, _ := os.UserHomeDir()
	for name, body := range map[string]string{
		"an Agent without a config directory of its own": "  x:\n    agent: cursor\n    config_dir: $ROOT/x\n",
		"an unknown Agent":                  "  x:\n    agent: nope\n    config_dir: $ROOT/x\n",
		"agent without config_dir":          "  x:\n    agent: claude\n",
		"config_dir without agent":          "  x:\n    config_dir: $ROOT/x\n",
		"a relative config_dir":             "  x:\n    agent: claude\n    config_dir: work\n",
		"the Agent's own default directory": "  x:\n    agent: claude\n    config_dir: " + filepath.Join(home, ".claude") + "\n",
		"two targets on one directory":      "  x:\n    agent: claude\n    config_dir: $ROOT/x\n  y:\n    agent: claude\n    config_dir: $ROOT/x\n",
		"the name of a built-in Agent":      "  claude:\n    agent: claude\n    config_dir: $ROOT/x\n",
	} {
		if _, _, err := loadWithTargets(t, body); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// cli names the executable that runs an account's plugin commands: a name found on PATH,
// or an absolute path.
func TestAgentConfigDir_CLI(t *testing.T) {
	home, _ := os.UserHomeDir()
	for cli, want := range map[string]string{
		"omo":              "omo",
		"/opt/omo/bin/omo": "/opt/omo/bin/omo",
		"~/bin/omo":        filepath.Join(home, "bin", "omo"),
	} {
		cfg, _, err := loadWithTargets(t, "  omo:\n    agent: pi\n    config_dir: $ROOT/.omo/agent\n    cli: "+cli+"\n")
		if err != nil {
			t.Fatalf("cli %s: %v", cli, err)
		}
		if got := cfg.Targets["omo"].CLI; got != want {
			t.Errorf("cli %s: got %q, want %q", cli, got, want)
		}
	}
}

func TestAgentConfigDir_CLIRejected(t *testing.T) {
	for name, body := range map[string]string{
		"cli on a target that is no account": "  x:\n    path: $ROOT/x\n    cli: omo\n",
		"a relative path":                    "  x:\n    agent: pi\n    config_dir: $ROOT/x\n    cli: bin/omo\n",
		"a command line":                     "  x:\n    agent: pi\n    config_dir: $ROOT/x\n    cli: omo --beta\n",
	} {
		if _, _, err := loadWithTargets(t, body); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// With preserve_tilde_on_save, cli is saved under ~ like the other paths.
func TestAgentConfigDir_SaveKeepsCLIUnderHome(t *testing.T) {
	cfg, _, err := loadWithTargets(t, "  omo:\n    agent: pi\n    config_dir: $ROOT/.omo/agent\n    cli: ~/bin/omo\npreserve_tilde_on_save: true\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(ConfigPath()); !strings.Contains(string(data), "cli: ~/bin/omo") {
		t.Fatalf("saved config:\n%s", data)
	}
}

// The config editor validates what it is about to save without loading it.
func TestAgentConfigDir_ConfigEditorValidation(t *testing.T) {
	source := t.TempDir()
	parse := func(body string) *Config {
		t.Helper()
		var cfg Config
		if err := yaml.Unmarshal([]byte("source: "+source+"\n"+body), &cfg); err != nil {
			t.Fatal(err)
		}
		return &cfg
	}
	account := "targets:\n  claude-work:\n    agent: claude\n    config_dir: " + filepath.Join(t.TempDir(), ".claude-work") + "\n"
	cfg := parse(account + "mcp:\n  targets: [claude, claude-work]\n")
	if _, err := ValidateConfig(cfg); err != nil {
		t.Fatalf("an account target was refused: %v", err)
	}
	if err := ValidateMCP(cfg.MCP, "", cfg.AgentConfigDirTargets()...); err != nil {
		t.Fatalf("an account was refused as an MCP target: %v", err)
	}
	if _, err := ValidateConfig(parse("targets:\n  cursor-work:\n    agent: cursor\n    config_dir: /tmp/cursor-work\n")); err == nil {
		t.Error("an Agent without a config directory was accepted")
	}
	for _, mcp := range []string{"mcp:\n  targets: [claude-home]\n", "mcp:\n  servers:\n    docs:\n      command: docs\n      targets: [claude-home]\n"} {
		if cfg := parse(account + mcp); ValidateMCP(cfg.MCP, "", cfg.AgentConfigDirTargets()...) == nil {
			t.Errorf("a name that is no target was accepted:\n%s", mcp)
		}
	}
}

// Codex keeps skills in the shared ~/.agents/skills, but an account's skills belong to
// that account's own directory, which is the path also_scans lists under it.
func TestAgentConfigDir_CodexSkillsFollowItsOwnDirectory(t *testing.T) {
	cfg, root, err := loadWithTargets(t, "  codex-work:\n    agent: codex\n    config_dir: $ROOT/.codex-work\n")
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["codex-work"]
	if got := target.SkillsConfig().Path; got != filepath.Join(root, ".codex-work", "skills") {
		t.Errorf("skills path %q", got)
	}
	if got := target.AgentsConfig().Path; got != "" {
		t.Errorf("Codex has no agents directory, got %q", got)
	}
}

func TestAgentConfigDir_PiSkillsFollowItsOwnDirectory(t *testing.T) {
	cfg, root, err := loadWithTargets(t, "  pi-work:\n    agent: pi\n    config_dir: $ROOT/.pi-work\n")
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["pi-work"]
	if got := target.SkillsConfig().Path; got != filepath.Join(root, ".pi-work", "skills") {
		t.Errorf("skills path %q", got)
	}
}
