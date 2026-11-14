package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"skillshare/internal/profile"
)

func TestProfilePaths_Identity(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("targets: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, lock, err := ProfilePaths(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(state)); !os.IsNotExist(err) {
		t.Fatalf("lookup created state directory: %v", err)
	}
	if enabled, err := ProfileConfigured(configPath, false); err != nil || enabled {
		t.Fatalf("empty config acquired locking boundary: %v, %v", enabled, err)
	}
	if enabled, err := ProfileConfigured(configPath, true); err != nil || !enabled {
		t.Fatalf("definitions did not acquire locking boundary: %v, %v", enabled, err)
	}
	if filepath.Ext(state) != ".yaml" || filepath.Ext(lock) != ".lock" {
		t.Fatalf("unexpected paths: %s, %s", state, lock)
	}
	other, _, err := ProfilePaths(filepath.Join(root, "another.yaml"))
	if err != nil || state == other {
		t.Fatalf("different configs share state: %s, %s, %v", state, other, err)
	}
	alias := filepath.Join(root, "alias.yaml")
	if err := os.Symlink(configPath, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	linkedState, linkedLock, err := ProfilePaths(alias)
	if err != nil || linkedState != state || linkedLock != lock {
		t.Fatalf("config alias did not share paths: %s, %s, %v", linkedState, linkedLock, err)
	}
}

func TestProfilePaths_ConfigOverrideAndWindowsState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "custom", "config.yaml"))
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	if got := projectStateDir("windows"); got != filepath.Join(root, "local", "skillshare", "state") {
		t.Fatalf("Windows local state: %s", got)
	}
	state, lock, err := ProfilePaths(ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	expected, expectedLock, err := ProfilePaths(filepath.Join(root, "custom", "config.yaml"))
	if err != nil || state != expected || lock != expectedLock {
		t.Fatalf("config override identity: %s, %s, %v", state, lock, err)
	}
}

func TestProfiles_RuntimeAttachment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	path := filepath.Join(root, "config.yaml")
	var cfg Config
	if err := yaml.Unmarshal([]byte("profiles:\n  web:\n    skills:\n      include: [web*]\ntargets:\n  claude:\n    path: /skills\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	statePath, _, err := ProfilePaths(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "web"}); err != nil {
		t.Fatal(err)
	}
	cfg.loadProfile(path)
	target := cfg.Targets["claude"]
	selected, err := target.SkillsConfig().Profile.Selects("web-ui")
	if err != nil || !selected {
		t.Fatalf("own target profile: %v, %v", selected, err)
	}
	selected, err = target.SkillsConfig().Profile.Selects("server")
	if err != nil || selected || target.AgentsConfig().Profile.Active() {
		t.Fatalf("profile leaked or filter failed: %v, %v", selected, err)
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Config
	if err := yaml.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	roundTarget := roundTrip.Targets["claude"]
	if roundTarget.SkillsConfig().Profile.Active() {
		t.Fatalf("runtime selector serialized: %s, %v", data, err)
	}
	if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "missing"}); err != nil {
		t.Fatal(err)
	}
	cfg.loadProfile(path)
	if !errors.Is(cfg.ActiveProfileError(), profile.ErrProfileUnavailable) {
		t.Fatalf("unavailable profile not reported: %v", cfg.ActiveProfileError())
	}
}

func TestProfiles_ProjectTargetAttachment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	path := ProjectConfigPath(root)
	var cfg ProjectConfig
	if err := yaml.Unmarshal([]byte("profiles:\n  web:\n    skills:\n      include: [web*]\ntargets:\n  - name: custom\n    skills:\n      path: .tool/skills\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	statePath, _, err := ProfilePaths(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "web", Pending: true}); err != nil {
		t.Fatal(err)
	}
	cfg.loadProfile(path)
	targets, err := ResolveProjectTargets(root, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	target := targets["custom"]
	selected, err := target.SkillsConfig().Profile.Selects("server")
	if err != nil || selected || !cfg.ProfileState().Pending {
		t.Fatalf("project selection or pending state: %v, %v, %+v", selected, err, cfg.ProfileState())
	}
}

func TestProfiles_CandidateDoesNotChangeLoadedConfig(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("profiles:\n  web:\n    skills:\n      include: [web*]\ntargets:\n  claude:\n    path: /skills\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Targets["project/claude"] = TargetConfig{projectRoot: "/project"}
	candidate := cfg.WithActiveProfile("web")
	if cfg.ProfileSelector().Active() || !candidate.ProfileSelector().Active() {
		t.Fatal("candidate mutated the loaded selection")
	}
	own := candidate.Targets["claude"]
	expanded := candidate.Targets["project/claude"]
	if ok, err := own.SkillsConfig().Profile.Selects("server"); err != nil || ok {
		t.Fatalf("candidate did not select web: %v, %v", ok, err)
	}
	if expanded.SkillsConfig().Profile.Active() {
		t.Fatal("global candidate reached a registered project")
	}
	delete(candidate.Targets, "claude")
	if _, ok := cfg.Targets["claude"]; !ok {
		t.Fatal("candidate shares target map")
	}
	missing := cfg.WithActiveProfile("missing")
	if !errors.Is(missing.ActiveProfileError(), profile.ErrProfileUnavailable) || cfg.ActiveProfileError() != nil {
		t.Fatal("candidate availability leaked into original")
	}
}
