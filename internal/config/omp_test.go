package config

import (
	"path/filepath"
	"testing"
)

func TestOMPAccountPaths(t *testing.T) {
	cfg, root, err := loadWithTargets(t, "  omp-work:\n    agent: omp\n    config_dir: $ROOT/.omp-work/agent\n")
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["omp-work"]
	if got := target.SkillsConfig().Path; got != filepath.Join(root, ".omp-work", "agent", "skills") {
		t.Fatalf("account skills: %q", got)
	}
	instructions, ok := TargetInstructions("omp-work", target, false)
	if !ok || instructions.Path != filepath.Join(target.ConfigDir, "AGENTS.md") {
		t.Fatalf("account instructions: %+v %t", instructions, ok)
	}
	fileRoot, ok := TargetFileRoot("omp-work", target, false, "")
	if !ok || fileRoot != target.ConfigDir {
		t.Fatalf("account file root: %q %t", fileRoot, ok)
	}
	project, ok := LookupProjectTarget("oh-my-pi")
	if !ok || project.Path != filepath.Join(".omp", "skills") {
		t.Fatalf("project skills alias: %+v %t", project, ok)
	}
}
