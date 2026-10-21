package projects

import (
	"bytes"
	"errors"
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

func TestServiceEdit_DetachesDepartingSkills(t *testing.T) {
	for _, change := range []string{"drop", "disable", "rename"} {
		t.Run(change, func(t *testing.T) {
			s, cfg, path, root, original := serviceFixture(t)
			serviceTargetManifests(t, cfg)
			next := cfg.Projects[root]
			want := 1
			switch change {
			case "drop":
				next.Targets = []string{"codex"}
			case "disable":
				next.Skills = nil
				want = 2
			case "rename":
				next.Name = "renamed"
				want = 0
			}
			preview, err := s.Put(root, next, PutOptions{DryRun: true, Detach: true})
			if err != nil {
				t.Fatal(err)
			}
			if want > 0 && (preview.Detach == nil || len(preview.Detach.Removed) != want) {
				t.Fatalf("preview lost departure: %+v", preview.Detach)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
				t.Fatal("preview wrote config")
			}
			result, err := s.Put(root, next, PutOptions{Detach: true})
			if err != nil {
				t.Fatal(err)
			}
			got := 0
			if result.Detach != nil {
				got = len(result.Detach.Removed)
			}
			if got != want {
				t.Fatalf("detached %d manifests, want %d", got, want)
			}
			if result.Detach != nil {
				for _, p := range result.Detach.Removed {
					if _, err := os.Lstat(p); !os.IsNotExist(err) {
						t.Fatalf("departing manifest retained: %s %v", p, err)
					}
				}
			}
		})
	}
}

func TestServiceEdit_SaveFailureReportsCompletedDetach(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	serviceTargetManifests(t, cfg)
	next := cfg.Projects[root]
	next.Targets = []string{"codex"}
	s.DetachSkillsTarget = func(target string, opts ssync.DetachOptions) (ssync.DetachReport, error) {
		r, err := ssync.DetachSkillsTarget(target, opts)
		if err == nil {
			if err := os.Rename(path, path+".kept"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0755); err != nil {
				t.Fatal(err)
			}
		}
		return r, err
	}
	result, err := s.Put(root, next, PutOptions{Detach: true})
	if !errors.Is(err, ErrSave) {
		t.Fatalf("want failed save: %v", err)
	}
	if result.Registration != "kept" || result.Detach == nil || len(result.Detach.Removed) != 1 || result.Item == nil || len(result.Item.Targets) != 2 {
		t.Fatalf("partial edit misrepresented persisted state: %+v", result)
	}
	if got, _ := os.ReadFile(path + ".kept"); !bytes.Equal(got, original) {
		t.Fatal("failed save changed config")
	}
}

func TestServiceEdit_AnchorDepartures(t *testing.T) {
	path, _, dir, _ := projectCandidateFixture(t, "  $ROOT/a: &shared\n    targets: [claude, codex]\n    skills: {}\n  $ROOT/z: *shared\n")
	for _, name := range []string{"a", "z"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	serviceTargetManifests(t, cfg)
	s := NewService(path, func() (*config.Config, error) { return cfg, nil })
	next := cfg.Projects[filepath.Join(dir, "a")]
	next.Targets = []string{"codex"}
	result, err := s.Put(filepath.Join(dir, "a"), next, PutOptions{Detach: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Affected) != 2 || result.Detach == nil || len(result.Detach.Removed) != 2 {
		t.Fatalf("inherited departing paths left managed: %+v", result)
	}
}
