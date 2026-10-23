package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
)

func TestExecuteTargets_CandidateRenameUsesExpandedTargets(t *testing.T) {
	source, skills := skillRunSource(t)
	root := t.TempDir()
	cfg := &config.Config{Source: source, Mode: "merge", Projects: map[string]config.ManagedProject{
		root: {Name: "original", Targets: []string{"claude"}, Skills: &config.ResourceTargetConfig{}},
	}}
	path := filepath.Join(t.TempDir(), "config.yaml")
	before, err := config.PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg = before.Expanded
	project := cfg.Projects[root]
	project.Name = "renamed"
	cfg.Projects[root] = project
	candidate, err := config.PrepareSave(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Targets["original@claude"]; !ok {
		t.Fatal("test requires stale expanded target before preparation")
	}
	got, err := ExecuteTargets(context.Background(), candidate.Expanded, candidate.Expanded.Targets, ExecuteOptions{Kinds: []string{"skill"}, Discovery: TargetDiscovery{Skills: skills}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Target != "renamed@claude" || !slices.Equal(got.Results[0].Linked, []string{"alpha"}) {
		t.Fatalf("renamed target and completed skill missing: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteTargets_MixedAndAllFailedRetainResults(t *testing.T) {
	for _, allFailed := range []bool{false, true} {
		t.Run(map[bool]string{false: "mixed", true: "all-failed"}[allFailed], func(t *testing.T) {
			source, skills := skillRunSource(t)
			targets := map[string]config.TargetConfig{
				"broken": {Skills: &config.ResourceTargetConfig{Path: t.TempDir(), Include: []string{"["}}},
				"good":   {Skills: &config.ResourceTargetConfig{Path: t.TempDir()}},
			}
			if allFailed {
				targets["good"] = targets["broken"]
			}
			got, err := ExecuteTargets(context.Background(), &config.Config{Source: source}, targets, ExecuteOptions{Kinds: []string{"skill"}, Discovery: TargetDiscovery{Skills: skills}})
			if err != nil {
				t.Fatal(err)
			}
			wantResults, wantFailed := 1, 1
			if allFailed {
				wantResults, wantFailed = 0, 2
			}
			if len(got.Results) != wantResults || len(got.Failed) != wantFailed || len(got.Warnings) != wantFailed {
				t.Fatalf("results/failures lost: %+v", got)
			}
			if !allFailed && (got.Results[0].Target != "good" || !slices.Equal(got.Results[0].Linked, []string{"alpha"})) {
				t.Fatalf("completed row lost: %+v", got)
			}
			for _, failed := range got.Failed {
				if failed.Part != "skill" || !strings.Contains(failed.Message, "sync failed") {
					t.Fatalf("failure detail lost: %+v", failed)
				}
			}
		})
	}
}

func TestExecuteTargets_FailedAgentsKeepCompletedSkills(t *testing.T) {
	source, skills := skillRunSource(t)
	agentsSource := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentsSource, "tutor.md"), []byte("# Tutor"), 0644); err != nil {
		t.Fatal(err)
	}
	agents, _, available := DiscoverTargetAgents(agentsSource)
	targets := map[string]config.TargetConfig{}
	for _, name := range []string{"one", "two"} {
		targets[name] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: t.TempDir()}, Agents: &config.ResourceTargetConfig{Path: t.TempDir(), Extension: "broken"}}
	}
	got, err := ExecuteTargets(context.Background(), &config.Config{Source: source}, targets, ExecuteOptions{
		AgentsSource: agentsSource, Discovery: TargetDiscovery{Skills: skills, Agents: agents, AgentsAvailable: available},
		ResolveExtension: func(string) (*ExtensionSpec, error) { return nil, fmt.Errorf("extension unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Failed) != 2 || len(got.Results) != 2 {
		t.Fatalf("failed targets lost completed skills: %+v", got)
	}
	for _, row := range got.Results {
		if !slices.Equal(row.Linked, []string{"alpha"}) {
			t.Fatalf("completed skill missing: %+v", row)
		}
	}
	for _, failed := range got.Failed {
		if failed.Part != "agent" {
			t.Fatalf("wrong failure part: %+v", failed)
		}
	}
}

func TestExecuteTargets_ExecutionOptionsAndStatistics(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		t.Run(fmt.Sprint(dryRun), func(t *testing.T) {
			source, skills := skillRunSource(t)
			target := filepath.Join(t.TempDir(), "skills")
			elsewhere := t.TempDir()
			if err := os.Symlink(elsewhere, target); err != nil {
				t.Fatal(err)
			}
			stats := &skillignore.IgnoreStats{IgnoredSkills: []string{"ignored"}}
			got, err := ExecuteTargets(context.Background(), &config.Config{Source: source}, map[string]config.TargetConfig{
				"linked": {Skills: &config.ResourceTargetConfig{Path: target, Mode: "symlink"}},
			}, ExecuteOptions{Kinds: []string{"skill"}, Force: true, DryRun: dryRun, Discovery: TargetDiscovery{Skills: skills, IgnoreStats: stats, Warnings: []string{"source notice"}}})
			if err != nil {
				t.Fatal(err)
			}
			wantResults, wantFailures := 1, 0
			// The existing symlink dry-run reports a conflicting location even with
			// force; execution replaces it. Preserve that runner contract.
			if dryRun {
				wantResults, wantFailures = 0, 1
			}
			if len(got.Results) != wantResults || len(got.Failed) != wantFailures || got.IgnoreStats != stats || len(got.Skills) != 1 || !slices.Contains(got.Warnings, "source notice") || got.FolderConflicts == nil || got.Unmatched == nil {
				t.Fatalf("options/statistics lost: %+v", got)
			}
			linked, err := os.Readlink(target)
			want := source
			if dryRun {
				want = elsewhere
			}
			if err != nil || linked != want {
				t.Fatalf("force/dry-run link = %q, %v; want %q", linked, err, want)
			}
		})
	}
}

func TestExecuteTargets_IncompleteDiscoveryPreservesUnmatchedSemantics(t *testing.T) {
	source, skills := skillRunSource(t)
	got, err := ExecuteTargets(context.Background(), &config.Config{Source: source}, map[string]config.TargetConfig{
		"filtered": {Skills: &config.ResourceTargetConfig{Path: t.TempDir(), Include: []string{"missing"}}},
	}, ExecuteOptions{Kinds: []string{"skill"}, Discovery: TargetDiscovery{Skills: skills, SourceIncomplete: true}})
	if err != nil || len(got.Unmatched) != 0 {
		t.Fatalf("incomplete source reported unmatched: %+v, %v", got, err)
	}
}

func TestExecuteTargets_NoEffectiveTargetsReturnsEmptyCollections(t *testing.T) {
	got, err := ExecuteTargets(context.Background(), &config.Config{}, map[string]config.TargetConfig{}, ExecuteOptions{})
	if err != nil || got.Results == nil || got.Failed == nil || got.Unmatched == nil || got.FolderConflicts == nil || len(got.Results)+len(got.Failed)+len(got.Unmatched)+len(got.FolderConflicts) != 0 {
		t.Fatalf("empty selection = %+v, %v", got, err)
	}
}

func TestExecuteTargets_ReturnsDiscoveredAgentsForIgnoreReport(t *testing.T) {
	source := t.TempDir()
	for _, name := range []string{"keep.md", "ignored.md"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("# Agent\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, ".agentignore"), []byte("ignored.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	agents, all, available := DiscoverTargetAgents(source)
	if !available || len(agents) != 1 || agents[0].RelPath != "keep.md" || len(all) != 2 {
		t.Fatalf("discovery = active %+v, all %+v, available %v", agents, all, available)
	}
	got, err := ExecuteTargets(context.Background(), &config.Config{}, map[string]config.TargetConfig{}, ExecuteOptions{
		Kinds: []string{"agent"}, DryRun: true, AgentsSource: source,
		Discovery: TargetDiscovery{Agents: agents, AllAgents: all, AgentsAvailable: available},
	})
	if err != nil {
		t.Fatal(err)
	}
	// A report built from the result must use the synced snapshot, not a rescan.
	if err := os.Remove(filepath.Join(source, "ignored.md")); err != nil {
		t.Fatal(err)
	}
	if r := NewIgnoreReport(nil, source, got.AllAgents); !slices.Equal(r.AgentIgnoredSkills, []string{"ignored.md"}) {
		t.Fatalf("ignore report rescanned the source: %+v", r)
	}
	t.Run("empty source", func(t *testing.T) {
		source := t.TempDir()
		agents, all, available := DiscoverTargetAgents(source)
		got, err := ExecuteTargets(context.Background(), &config.Config{}, map[string]config.TargetConfig{}, ExecuteOptions{
			Kinds: []string{"agent"}, DryRun: true, AgentsSource: source,
			Discovery: TargetDiscovery{Agents: agents, AllAgents: all, AgentsAvailable: available},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !available || got.AllAgents == nil || len(got.AllAgents) != 0 {
			t.Fatalf("empty source lost the non-nil snapshot: available %v, AllAgents %#v", available, got.AllAgents)
		}
	})
}
