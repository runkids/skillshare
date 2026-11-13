package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func runTestOptions() ExtraRunOptions {
	return ExtraRunOptions{
		ResolvePath: func(p string) string { return p },
		ResolveExtension: func(ext string) (*ExtensionSpec, error) {
			return nil, errors.New("no extension " + ext)
		},
	}
}

func TestRunExtraTargets_MissingSourceSkip(t *testing.T) {
	t.Run("interruption policy", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			for _, remote := range []bool{false, true} {
				for _, live := range []bool{false, true} {
					for _, targetless := range []bool{false, true} {
						t.Run(fmt.Sprintf("project=%t/remote=%t/live=%t/targetless=%t", project, remote, live, targetless), func(t *testing.T) {
							root := t.TempDir()
							source := filepath.Join(root, "rules")
							target := t.TempDir()
							configPath := filepath.Join(root, "config.yaml")
							if err := os.WriteFile(configPath, []byte("unchanged config"), 0600); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(filepath.Join(target, "sentinel"), []byte("target before"), 0644); err != nil {
								t.Fatal(err)
							}
							if live {
								if err := os.Mkdir(source, 0755); err != nil {
									t.Fatal(err)
								}
								if err := os.WriteFile(filepath.Join(source, "rule.md"), []byte("source before"), 0644); err != nil {
									t.Fatal(err)
								}
							}
							stage := filepath.Join(root, ".skillshare-staging")
							if err := os.Mkdir(stage, 0700); err != nil {
								t.Fatal(err)
							}
							marker := filepath.Join(stage, "publishing-rules.json")
							if err := os.WriteFile(marker, []byte("opaque interrupted state"), 0600); err != nil {
								t.Fatal(err)
							}
							entry := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: target, Mode: "copy"}}}
							if remote {
								entry.Remote = "example-org/ai-hub"
							}
							if targetless {
								entry.Targets = nil
							}
							opts := runTestOptions()
							if project {
								opts.ProjectRoot = root
							}
							got := RunExtraTargets(entry, source, opts)
							if remote {
								want := "extra \"rules\" has an interrupted publication; run: " + config.ExtraRestoreCommand(entry, project)
								if len(got.Warnings) != 1 || got.Warnings[0] != want || got.SourceMissing || len(got.Targets) != 0 {
									t.Fatalf("interruption not refused before targets: %+v", got)
								}
								if _, err := os.Lstat(filepath.Join(target, "rule.md")); !os.IsNotExist(err) {
									t.Fatalf("interrupted target written: %v", err)
								}
							} else if len(got.Warnings) != 0 || got.SourceMissing == live || (live && !targetless && (len(got.Targets) != 1 || got.Targets[0].Err != nil || got.Targets[0].Result == nil)) {
								t.Fatalf("local marker changed ordinary sync: %+v", got)
							}
							for path, want := range map[string]string{marker: "opaque interrupted state", configPath: "unchanged config", filepath.Join(target, "sentinel"): "target before"} {
								if body, err := os.ReadFile(path); err != nil || string(body) != want {
									t.Fatalf("changed %s: %q %v", path, body, err)
								}
							}
							if live {
								if body, err := os.ReadFile(filepath.Join(source, "rule.md")); err != nil || string(body) != "source before" {
									t.Fatalf("source changed: %q %v", body, err)
								}
							} else if _, err := os.Lstat(source); !os.IsNotExist(err) {
								t.Fatalf("source created: %v", err)
							}
							if _, err := os.Lstat(filepath.Join(root, ".skillshare-extras.lock")); !os.IsNotExist(err) {
								t.Fatalf("read-only sync acquired root lock: %v", err)
							}
							// The same source without a marker is an ordinary remote/local control.
							if err := os.Remove(marker); err != nil {
								t.Fatal(err)
							}
							ordinary := RunExtraTargets(entry, source, opts)
							if len(ordinary.Warnings) != 0 || ordinary.SourceMissing == live {
								t.Fatalf("ordinary sync misclassified: %+v", ordinary)
							}
							if live && !targetless {
								if len(ordinary.Targets) != 1 || ordinary.Targets[0].Err != nil {
									t.Fatalf("ordinary target failed: %+v", ordinary)
								}
								if body, err := os.ReadFile(filepath.Join(target, "rule.md")); err != nil || !strings.Contains(string(body), "source before") {
									t.Fatalf("ordinary target not delivered: %q %v", body, err)
								}
							}
						})
					}
				}
			}
		}
	})
	src := filepath.Join(t.TempDir(), "missing")
	extra := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: t.TempDir()}}}

	run := RunExtraTargets(extra, src, runTestOptions())

	if !run.SourceMissing || len(run.Targets) != 0 {
		t.Fatalf("expected skipped extra, got %+v", run)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source must not be created, stat err = %v", err)
	}
}

func TestRunExtraTargets_SkipsAgentOverlap(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{"a.md": "a"})
	extra := config.ExtraConfig{Name: "agents", Targets: []config.ExtraTargetConfig{{Path: tgt}}}
	opts := runTestOptions()
	opts.AgentTargetPaths = map[string]bool{filepath.Clean(tgt): true}

	run := RunExtraTargets(extra, src, opts)

	if got := run.Targets[0]; got.SkippedBy != "agents" || got.Result != nil {
		t.Fatalf("expected target skipped by agents, got %+v", got)
	}
}

// Only the "agents" extra's target at an agents sync path is skipped.
func TestRunExtraTargets_AgentOverlapIsPerTargetAndPerName(t *testing.T) {
	src, overlap := setupExtrasTest(t, map[string]string{"a.md": "a"})
	other := t.TempDir()
	opts := runTestOptions()
	opts.AgentTargetPaths = map[string]bool{filepath.Clean(overlap): true}
	targets := []config.ExtraTargetConfig{{Path: overlap}, {Path: other}}

	agents := RunExtraTargets(config.ExtraConfig{Name: "agents", Targets: targets}, src, opts).Targets
	if agents[0].SkippedBy != "agents" || agents[1].SkippedBy != "" || agents[1].Result == nil {
		t.Errorf("agents extra: want only the overlapping target skipped, got %+v", agents)
	}
	rules := RunExtraTargets(config.ExtraConfig{Name: "rules", Targets: targets[:1]}, src, opts).Targets
	if rules[0].SkippedBy != "" || rules[0].Result == nil {
		t.Errorf("rules extra at an agents path must sync, got %+v", rules[0])
	}
}

func TestAgentTargetPaths(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "agents")

	if got := AgentTargetPaths(source, []string{target}); got != nil {
		t.Errorf("no agents in the source: want nil, got %v", got)
	}

	if err := os.WriteFile(filepath.Join(source, "helper.md"), []byte("# Helper"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := AgentTargetPaths(source, []string{target + "/", ""})
	if len(got) != 1 || !got[target] {
		t.Errorf("want only the cleaned %s, got %v", target, got)
	}
}

func TestRunExtraTargets_ReportsModeAndExtensionErrors(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{"a.md": "a"})
	extra := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: tgt, Mode: "merge", Extension: "x"}}}

	got := RunExtraTargets(extra, src, runTestOptions()).Targets[0]

	if got.ModeErr == nil || got.ExtensionErr == nil || got.Result != nil || got.Mode != "merge" {
		t.Fatalf("expected both errors, no sync and configured mode, got %+v", got)
	}
}
