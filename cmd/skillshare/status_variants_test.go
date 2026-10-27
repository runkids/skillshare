package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
)

func TestTrackedRepoStatusCountsLogicalVariants(t *testing.T) {
	src := t.TempDir()
	for _, agent := range []string{"claude", "codex"} {
		for _, name := range []string{"fix", "review/fix"} {
			dir := filepath.Join(src, "_repo/internal/skills", agent, name)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Instructions"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	initGitRepoForTest(t, filepath.Join(src, "_repo"))
	skills, trackedRepos, err := sync.DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	repos := buildTrackedRepoJSON(src, trackedRepos, skills)
	if len(repos) != 1 || repos[0].SkillCount != 2 {
		t.Fatalf("repository counts = %+v, want 2 logical skills", repos)
	}
	if repos[0].Status != "" || repos[0].Dirty {
		t.Fatalf("tracked fixture must be a clean Git repository: %+v", repos[0])
	}
	if repos[0].VariantCount != 4 {
		t.Fatalf("physical count = %d, want 4", repos[0].VariantCount)
	}
	if text := sourceSkillCountText(repos[0].SkillCount, repos[0].VariantCount); text != "2 skills, 4 variants" {
		t.Fatalf("status text = %q", text)
	}
	doctor := &doctorResult{}
	output := captureStdout(t, func() { checkSource(&config.Config{Source: src}, sourcewalk.Options{}, doctor, skills, nil) })
	if !strings.Contains(output, "2 skills, 4 variants") || !strings.Contains(doctor.checks[0].Message, "2 skills, 4 variants") {
		t.Errorf("doctor disagrees with inventory: %s, %+v", output, doctor.checks)
	}
	for _, name := range []string{"XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", configPath)
	if err := os.WriteFile(configPath, []byte("source: "+src+"\nmode: merge\ntargets:\n  claude:\n    path: "+filepath.Join(t.TempDir(), "skills")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-g", "--dry-run"}, {"-g"}} {
		var syncErr error
		output := stripANSIWarnings(captureStdout(t, func() { syncErr = cmdSync(args) }))
		if syncErr != nil {
			t.Fatal(syncErr)
		}
		if !strings.Contains(output, "2 skills, 4 variants to 1 target") {
			t.Fatalf("sync summary must count source inventory: %s", output)
		}
	}
	for _, suffix := range []string{"fix", "review/fix"} {
		dir := filepath.Join(src, "_repo/internal/skills/claude-code", suffix)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Instructions"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, flag := range []string{"--json", "--no-tui"} {
		var diffErr error
		output := captureStdout(t, func() { diffErr = cmdDiff([]string{"-g", flag}) })
		if diffErr != nil {
			t.Fatal(diffErr)
		}
		if !strings.Contains(output, "ambiguous") || !strings.Contains(output, "target override") {
			t.Fatalf("diff lost selection reason: %s", output)
		}
		if flag == "--json" {
			var result diffJSONOutput
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Targets) != 1 || result.Targets[0].Synced {
				t.Fatalf("warning-only target must require attention: %s", output)
			}
		}
	}
}

func TestDiffLocalOnlyFoldersStayInSync(t *testing.T) {
	result := targetDiffResult{localCount: 1, items: []copyDiffEntry{{name: "personal", action: "remove", reason: "local only"}}}
	if !result.inSync() {
		t.Fatal("a local-only folder is not pending sync work")
	}
}
