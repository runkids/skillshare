package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
)

func profileDiagnosticFixture(t *testing.T, project bool, definitions string, state profile.State) (*config.Config, *config.ProjectConfig, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	for _, name := range []string{"keep", "edit"} {
		dir := filepath.Join(source, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "config.yaml")
	text := fmt.Sprintf("source: %s\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: %s\n  linked:\n    skills:\n      path: %s\n      mode: symlink\n", source, target, target+"-link")
	if project {
		path = config.ProjectConfigPath(root)
		text = fmt.Sprintf("sources:\n  skills: %s\ntargets:\n  - name: custom\n    skills:\n      path: %s\n      mode: copy\n  - name: linked\n    skills:\n      path: %s\n      mode: symlink\n", source, target, target+"-link")
	}
	t.Setenv("SKILLSHARE_CONFIG", path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text+definitions), 0600); err != nil {
		t.Fatal(err)
	}
	statePath, _, err := config.ProfilePaths(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.WriteState(statePath, state); err != nil {
		t.Fatal(err)
	}
	if project {
		pc, err := config.LoadProjectReadOnly(root)
		if err != nil {
			t.Fatal(err)
		}
		targets, _ := config.ResolveValidProjectTargets(root, pc)
		return &config.Config{Source: source, Targets: targets, Mode: "merge", Profiles: pc.Profiles}, pc, root
	}
	cfg, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, nil, root
}

func TestProfileDoctorChecks(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			cfg, pc, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: [keep, missing]\n  empty:\n    skills:\n      include: []\n      exclude: [edit]\n", profile.State{Version: 1, Active: "work", Pending: true})
			skills, err := ssync.DiscoverSourceSkills(cfg.Source)
			if err != nil {
				t.Fatal(err)
			}
			target := cfg.Targets["custom"]
			sc := target.SkillsConfig()
			sc.Profile = profile.Selector{}
			target = config.TargetConfig{Skills: &sc}
			initial := ssync.SyncSkillTarget(ssync.SkillTarget{Name: "custom", Target: target, Mode: "copy"}, skills, ssync.SkillRunOptions{Source: cfg.Source})
			if initial.Err != nil || len(initial.Linked) != 2 {
				t.Fatal("fixture must copy both skills before activating its profile", initial)
			}
			edited := filepath.Join(sc.Path, "edit", "SKILL.md")
			if err := os.WriteFile(edited, []byte("my edit"), 0600); err != nil {
				t.Fatal(err)
			}
			checks := profileDoctorChecks(cfg, pc, root, skills, nil)
			result := &doctorResult{checks: checks}
			data, err := json.Marshal(buildDoctorOutput(result))
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"profile_pending", "profile_kept_modified", "profile_unmatched", "profile_inert_excludes", "profile_symlink_target"} {
				if !strings.Contains(string(data), `"name":"`+name+`"`) {
					t.Fatalf("missing %s: %s", name, data)
				}
			}
			if text, err := os.ReadFile(edited); err != nil || string(text) != "my edit" {
				t.Fatal("doctor changed target", string(text), err)
			}
			path := config.ConfigPath()
			if project {
				path = config.ProjectConfigPath(root)
			}
			_, lockPath, err := config.ProfilePaths(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
				t.Fatal("doctor created a profile lock", err)
			}
		})
	}
	for _, active := range []string{"missing", "broken"} {
		t.Run(active, func(t *testing.T) {
			cfg, pc, root := profileDiagnosticFixture(t, false, "profiles:\n  broken:\n    skills:\n      exclude: ['*']\n", profile.State{Version: 1, Active: active})
			checks := profileDoctorChecks(cfg, pc, root, nil, nil)
			if len(checks) != 1 || checks[0].Name != "profile_unavailable" || checks[0].Status != checkError {
				t.Fatal(checks)
			}
		})
	}
}

func TestProfileDiagnosticsWithoutActiveProfile(t *testing.T) {
	cfg, pc, root := profileDiagnosticFixture(t, false, "", profile.State{Version: 1})
	checks := profileDoctorChecks(cfg, pc, root, nil, nil)
	if len(checks) != 0 || profileUnmatchedWarning(profile.Parse(&cfg.Profiles), cfg.ProfileState(), nil) != "" {
		t.Fatal("no-profile diagnostics changed", checks)
	}
}
