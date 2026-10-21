package projects

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

func TestServicePut_KeepsDepartingSkills(t *testing.T) {
	for _, change := range []string{"drop", "disable"} {
		t.Run(change, func(t *testing.T) {
			s, cfg, _, root, _ := serviceFixture(t)
			manifests := serviceTargetManifests(t, cfg)
			s.DetachSkillsTarget = func(path string, _ ssync.DetachOptions) (ssync.DetachReport, error) {
				t.Fatalf("save detached %s", path)
				return ssync.DetachReport{}, nil
			}
			next := cfg.Projects[root]
			if change == "drop" {
				next.Targets = []string{"codex"}
			} else {
				next.Skills = nil
			}
			result, err := s.Put(root, next, PutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if result.Registration != "updated" || result.Detach != nil {
				t.Fatalf("save result: %+v", result)
			}
			for _, p := range manifests {
				if _, err := os.Stat(p); err != nil {
					t.Fatalf("save removed manifest %s: %v", p, err)
				}
			}
		})
	}
}

// An agents-only project can still hold manifests from a sync made while its
// skills were on, or with exclude: ["*"]. Remove detaches them, as on main.
func TestServiceRemove_SkillsOffProjectDetaches(t *testing.T) {
	path, _, dir, _ := projectCandidateFixture(t, "  $ROOT/app:\n    targets: [claude, codex]\n    agents: {}\n")
	root := filepath.Join(dir, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	manifests := serviceTargetManifests(t, cfg)
	if len(manifests) == 0 {
		t.Fatal("fixture has no project targets")
	}
	s := NewService(path, func() (*config.Config, error) { return cfg, nil })
	result, err := s.Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Detach == nil || len(result.Detach.Removed) != len(manifests) {
		t.Fatalf("skills-off project left managed: %+v", result.Detach)
	}
	for _, p := range manifests {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("manifest retained %s: %v", p, err)
		}
	}
}

func TestServiceRemove_SharedPathRetained(t *testing.T) {
	s, cfg, _, root, _ := serviceFixture(t)
	manifests := serviceTargetManifests(t, cfg)
	shared := filepath.Join(root, ".claude", "skills")
	cfg.Targets["shared"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: shared}}
	result, err := s.Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Detach == nil || len(result.Detach.Retained) != 1 || result.Detach.Retained[0] != shared {
		t.Fatalf("shared path not reported: %+v", result.Detach)
	}
	for _, p := range manifests {
		if filepath.Dir(p) == shared {
			if _, err := os.Stat(p); err != nil {
				t.Fatal("surviving target lost manifest", err)
			}
		}
	}
}
