package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSharedSkillsTargetsResolveAliasesAndKeepConflicts(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skip(err)
	}
	for _, incompatible := range []bool{false, true} {
		targets := map[string]TargetConfig{
			"codex":     {Path: filepath.Join(root, "skills")},
			"universal": {Path: filepath.Join(alias, "skills")},
		}
		if incompatible {
			tc := targets["universal"]
			tc.Exclude = []string{"*"}
			targets["universal"] = tc
		}
		resolved := WithSharedSkillsTargets(targets, "merge")
		for name, tc := range resolved {
			want := 2
			if incompatible {
				want = 0
			}
			if len(tc.SkillsConfig().SharedTargets) != want {
				t.Fatalf("%s incompatible=%v: %+v", name, incompatible, tc.SkillsConfig())
			}
			original := targets[name]
			if len(original.SkillsConfig().SharedTargets) != 0 {
				t.Fatal("mutated caller's configuration")
			}
		}
	}
}

func TestManagedProjectSharedSkillsKeepEveryToolIdentity(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{Projects: map[string]ManagedProject{root: {Targets: []string{"codex", "universal"}, Skills: &ResourceTargetConfig{}}}}
	if err := cfg.expandProjects(); err != nil {
		t.Fatal(err)
	}
	resolved := WithSharedSkillsTargets(cfg.Targets, "merge")
	if len(resolved) != 1 {
		t.Fatalf("project should have one writer: %+v", resolved)
	}
	for _, tc := range resolved {
		if len(tc.SkillsConfig().SharedTargets) != 2 {
			t.Fatalf("group lost identities: %+v", tc.SkillsConfig())
		}
	}
}
