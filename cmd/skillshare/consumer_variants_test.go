package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sync"
)

func consumerVariantSource(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	for _, agent := range []string{"claude", "codex"} {
		dir := filepath.Join(source, "_repo", "skills", agent, "fix")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Fix instructions\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	initGitRepoForTest(t, filepath.Join(source, "_repo"))
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	for _, key := range []string{"XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(key, t.TempDir())
	}
	return source
}

func TestConsumerVariantInventorySummaries(t *testing.T) {
	source := consumerVariantSource(t)
	discovered, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() { displayTrackedRepos([]string{"_repo"}, discovered, source) })
	if !strings.Contains(output, "1 skill, 2 variants") {
		t.Errorf("list summary: %s", output)
	}
	cfg := &config.Config{Source: source, AgentsSource: t.TempDir(), Mode: "merge", Targets: map[string]config.TargetConfig{}}
	output = captureStdout(t, func() {
		count, _, err := firstSync(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("firstSync count = %d, want logical 1", count)
		}
	})
	if !strings.Contains(output, "1 skill, 2 variants to 0 tools") {
		t.Errorf("init summary: %s", output)
	}
	parsed, err := install.ParseSource(filepath.Join(source, "_repo"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Source = t.TempDir()
	output = captureStdout(t, func() {
		if _, err := handleTrackedRepoInstall(parsed, cfg, install.InstallOptions{Kind: "skill"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "1 skill, 2 variants") {
		t.Errorf("install summary: %s", output)
	}
}

func TestConsumerVariantSymlinkDiagnostics(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			source := consumerVariantSource(t)
			cfg := &config.Config{Source: source, AgentsSource: t.TempDir(), Mode: "symlink", Targets: map[string]config.TargetConfig{
				"claude": {Path: filepath.Join(t.TempDir(), "skills")},
			}}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			if project {
				pc := &config.ProjectConfig{Sources: config.ProjectSources{Skills: source}, Targets: []config.ProjectTargetEntry{{Name: "claude", Path: cfg.Targets["claude"].Path, Mode: "symlink"}}}
				if err := pc.Save(root); err != nil {
					t.Fatal(err)
				}
			}
			const want = "symlink mode exposes all agent variants"
			output := captureStdout(t, func() {
				var err error
				if project {
					err = cmdStatusProject(root)
				} else {
					err = printTargetsStatus(cfg, mustDiscoverConsumerVariants(t, source))
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			if !strings.Contains(output, want) {
				t.Errorf("status diagnostic: %s", output)
			}
			output = captureStdout(t, func() {
				var err error
				if project {
					err = cmdStatusProjectJSON(root)
				} else {
					err = cmdStatus([]string{"-g", "--json"})
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			var status statusJSONOutput
			if err := json.Unmarshal([]byte(output), &status); err != nil {
				t.Fatal(err)
			}
			if status.SkillCount != 1 || status.VariantCount != 2 {
				t.Errorf("JSON inventory = %d/%d, want 1/2", status.SkillCount, status.VariantCount)
			}
			if len(status.Targets) != 1 || !strings.Contains(status.Targets[0].Warning, want) {
				t.Errorf("JSON diagnostic: %s", output)
			}
			result := &doctorResult{}
			captureStdout(t, func() { checkSyncDrift(cfg, result, mustDiscoverConsumerVariants(t, source), nil) })
			if len(result.checks) < 1 || !strings.Contains(strings.Join(result.checks[0].Details, " "), want) {
				t.Errorf("doctor diagnostic: %+v", result.checks)
			}
		})
	}
}

func mustDiscoverConsumerVariants(t *testing.T, source string) []sync.DiscoveredSkill {
	t.Helper()
	skills, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	return skills
}

func TestConsumerVariantDiagnosticScope(t *testing.T) {
	disabled := false
	for _, tc := range []struct {
		name   string
		target config.TargetConfig
		mode   string
		skills []sync.DiscoveredSkill
	}{
		{name: "merge", mode: "merge", skills: []sync.DiscoveredSkill{{VariantAgent: "codex"}}},
		{name: "copy", mode: "copy", skills: []sync.DiscoveredSkill{{VariantAgent: "codex"}}},
		{name: "ordinary", mode: "symlink", skills: []sync.DiscoveredSkill{{RelPath: "ordinary"}}},
		{name: "skills off", mode: "symlink", target: config.TargetConfig{Skills: &config.ResourceTargetConfig{Enabled: &disabled}}, skills: []sync.DiscoveredSkill{{VariantAgent: "codex"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := variantSymlinkWarning(tc.target, tc.mode, tc.skills); got != "" {
				t.Errorf("unexpected warning: %s", got)
			}
		})
	}
}

func TestFirstSyncSharedVariants(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			source := consumerVariantSource(t)
			dest := t.TempDir()
			cfg := &config.Config{Source: source, AgentsSource: t.TempDir(), Mode: mode, Targets: map[string]config.TargetConfig{
				"codex": {Path: dest}, "universal": {Path: dest},
			}}
			captureStdout(t, func() {
				if _, _, err := firstSync(cfg); err != nil {
					t.Fatal(err)
				}
			})
			if _, err := os.Stat(filepath.Join(dest, "_repo__skills__codex__fix", "SKILL.md")); err != nil {
				t.Fatalf("init pruned shared Codex variant: %v", err)
			}
		})
	}
}

func TestIgnoredVariantsSymlinkWarning(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			source := consumerVariantSource(t)
			if err := os.WriteFile(filepath.Join(source, ".skillignore"), []byte("**\n"), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{Source: source, AgentsSource: t.TempDir(), Mode: "symlink", Targets: map[string]config.TargetConfig{"claude": {Path: filepath.Join(t.TempDir(), "skills")}}}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			if project {
				pc := &config.ProjectConfig{Sources: config.ProjectSources{Skills: source}, Targets: []config.ProjectTargetEntry{{Name: "claude", Path: cfg.Targets["claude"].Path, Mode: "symlink"}}}
				if err := pc.Save(root); err != nil {
					t.Fatal(err)
				}
			}
			output := captureStdout(t, func() {
				var err error
				if project {
					err = cmdStatusProjectJSON(root)
				} else {
					err = cmdStatus([]string{"-g", "--json"})
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			var status statusJSONOutput
			if err := json.Unmarshal([]byte(output), &status); err != nil {
				t.Fatal(err)
			}
			if status.SkillCount != 0 || status.VariantCount != 0 {
				t.Fatalf("ignored inventory counted: %s", output)
			}
			if len(status.Targets) != 1 || !strings.Contains(status.Targets[0].Warning, "symlink mode exposes all agent variants") {
				t.Fatalf("ignored variants still exposed without warning: %s", output)
			}
			discovered, err := sync.DiscoverSourceSkills(source)
			if err != nil {
				t.Fatal(err)
			}
			result := &doctorResult{}
			captureStdout(t, func() { checkVariantRouting(cfg, result, discovered) })
			if len(result.checks) != 1 || result.checks[0].Status != checkWarning {
				t.Fatalf("doctor missed ignored variants: %+v", result)
			}
		})
	}
}
