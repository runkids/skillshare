package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

func TestHandleGitInstall_SubdirPlan(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "collection"}[nested], func(t *testing.T) {
			repo := t.TempDir()
			subdir := ".claude/skills/explicit"
			paths := []string{"."}
			want := []installPlannedSkill{{Name: "explicit", Path: subdir}}
			if nested {
				paths = []string{"a", "b"}
				want = []installPlannedSkill{{Name: "a", Path: subdir + "/a"}, {Name: "b", Path: subdir + "/b"}}
			}
			for _, path := range paths {
				dir := filepath.Join(repo, subdir, path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			initGitRepoForTest(t, repo)
			source := &install.Source{Type: install.SourceTypeGitHTTPS, Raw: "file://" + repo + "/" + subdir, CloneURL: "file://" + repo, Subdir: subdir, Name: "explicit"}
			cfg := &config.Config{Source: t.TempDir()}
			captureStdoutStderr(t, func() {
				summary, err := handleGitInstall(source, cfg, install.InstallOptions{DryRun: true, All: true, Force: true})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(summary.PlannedSkills, want) {
					t.Fatalf("planned = %+v, want %+v", summary.PlannedSkills, want)
				}
			})
		})
	}
}
