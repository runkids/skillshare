package main

import (
	"os"
	"path/filepath"
	"testing"

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
	skills, _, err := sync.DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	repos := buildTrackedRepoJSON(src, []string{"_repo"}, skills)
	if len(repos) != 1 || repos[0].SkillCount != 2 {
		t.Fatalf("repository counts = %+v, want 2 logical skills", repos)
	}
	if repos[0].VariantCount != 4 {
		t.Fatalf("physical count = %d, want 4", repos[0].VariantCount)
	}
	if text := sourceSkillCountText(repos[0].SkillCount, repos[0].VariantCount); text != "2 skills, 4 variants" {
		t.Fatalf("status text = %q", text)
	}
}
