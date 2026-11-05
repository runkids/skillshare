package projects

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

// A registration must not republish stale shared intent (hooks, targets, etc.).
func TestLocalRegistrationNeverWritesSharedConfig(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	next := cfg.Projects[root]
	next.Name = "local-name"
	result, err := s.Put(root, next, PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("registration rewrote shared config")
	}
	loaded, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Projects[root].Name != "local-name" {
		t.Fatalf("local registration missing: %+v", loaded.Projects)
	}
	s = NewService(path, func() (*config.Config, error) { return loaded, nil })
	if _, err := s.Remove(root, RemoveOptions{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("removal rewrote shared config")
	}
	loaded, err = config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Projects[root]; ok {
		t.Fatal("removed shared registration reappeared")
	}
	if result.Registration != "updated" {
		t.Fatal(result.Registration)
	}
}

func TestLocalMutationRejectsStaleSharedBeforeDetach(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "edit", true: "remove"}[remove], func(t *testing.T) {
			s, cfg, path, root, original := serviceFixture(t)
			if err := os.WriteFile(path, append(original, []byte("# newer shared settings\n")...), 0644); err != nil {
				t.Fatal(err)
			}
			calls := 0
			s.DetachSkillsTarget = func(string, ssync.DetachOptions) (ssync.DetachReport, error) {
				calls++
				return ssync.DetachReport{}, nil
			}
			var err error
			if remove {
				_, err = s.Remove(root, RemoveOptions{})
			} else {
				p := cfg.Projects[root]
				p.Name = "renamed"
				_, err = s.Put(root, p, PutOptions{Detach: true})
			}
			if err == nil || calls != 0 {
				t.Fatalf("stale operation: error=%v detach calls=%d", err, calls)
			}
		})
	}
}

func TestLocalRegistrationPrivateState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	s, cfg, path, root, _ := serviceFixture(t)
	release, err := LockConfig(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	p := cfg.Projects[root]
	p.Name = "local"
	if _, err := s.Put(root, p, PutOptions{}); err != nil {
		t.Fatal(err)
	}
	local, err := config.LocalProjectsPath(path)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{local: 0600, filepath.Dir(local): 0700} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s has %o, want %o", p, info.Mode().Perm(), want)
		}
	}
}

func TestLocalNoopEmptyFiltersKeepsSharedOrigin(t *testing.T) {
	s, cfg, path, root, _ := serviceFixture(t)
	p := cfg.Projects[root]
	p.Skills.Include = []string{}
	// This shape is emitted by the supported raw shared editor.
	cfg.Projects[root] = p
	p.Skills = cloneProjectResource(p.Skills)
	p.Skills.Include = nil
	result, err := s.Put(root, p, PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Affected) != 0 || result.Item.Origin != "shared" {
		t.Fatalf("no-op changed provenance: %+v", result)
	}
	local, err := config.LocalProjectsPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatalf("no-op created local state: %v", err)
	}
}
