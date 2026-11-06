package projects

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

func reload(t *testing.T, path string) (*Service, *config.Config) {
	t.Helper()
	loaded, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(path, func() (*config.Config, error) { return loaded, nil }), loaded
}

// An edit of a machine-local project must not republish config.yaml.
func TestLocalEditNeverWritesSharedConfig(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	if _, err := s.Localize(root, false); err != nil {
		t.Fatal(err)
	}
	s, cfg = reload(t, path)
	next := cfg.Projects[root]
	next.Name = "local-name"
	result, err := s.Put(root, next, PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("local edit rewrote shared config")
	}
	if result.Registration != "updated" || result.Item.Name != "local-name" || result.Item.Origin != OriginLocal || !result.Item.SharedCopy {
		t.Fatalf("local edit result: %+v", result.Item)
	}
}

// An edit of a config.yaml project is saved in config.yaml.
func TestSharedEditWritesSharedConfig(t *testing.T) {
	s, cfg, path, root, _ := serviceFixture(t)
	next := cfg.Projects[root]
	next.Name = "renamed"
	if _, err := s.Put(root, next, PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Contains(got, []byte("name: renamed")) {
		t.Fatalf("shared edit missing from config.yaml:\n%s", got)
	}
	local, _ := config.LocalProjectsPath(path)
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatalf("shared edit created local state: %v", err)
	}
}

// Removing this machine's copy brings back the config.yaml declaration.
func TestRemoveLocalCopyKeepsSharedDeclaration(t *testing.T) {
	s, cfg, path, root, original := serviceFixture(t)
	if _, err := s.Localize(root, false); err != nil {
		t.Fatal(err)
	}
	s, cfg = reload(t, path)
	result, err := s.Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("removing the local copy rewrote config.yaml")
	}
	if result.Remaining == nil || result.Remaining.Origin != OriginShared {
		t.Fatalf("remaining declaration: %+v", result.Remaining)
	}
	_, loaded := reload(t, path)
	if origin, _ := loaded.ProjectOrigin(root); origin != OriginShared || !config.SameManagedProject(loaded.Projects[root], cfg.Projects[root]) {
		t.Fatalf("shared declaration did not apply again: %s %+v", origin, loaded.Projects)
	}
}

// Removing with Shared drops both declarations.
func TestRemoveSharedDropsBothLayers(t *testing.T) {
	s, _, path, root, _ := serviceFixture(t)
	if _, err := s.Localize(root, false); err != nil {
		t.Fatal(err)
	}
	s, _ = reload(t, path)
	if _, err := s.Remove(root, RemoveOptions{Shared: true}); err != nil {
		t.Fatal(err)
	}
	_, loaded := reload(t, path)
	if len(loaded.Projects) != 0 || loaded.HasSharedProject(root) {
		t.Fatalf("project survived: %+v", loaded.Projects)
	}
}

// Share moves a local project into config.yaml and drops the local copy.
func TestShareMovesProjectIntoConfig(t *testing.T) {
	s, cfg, path, root, _ := serviceFixture(t)
	p := cfg.Projects[root]
	p.Name = "machine"
	if _, err := s.Localize(root, false); err != nil {
		t.Fatal(err)
	}
	s, cfg = reload(t, path)
	if _, err := s.Put(root, p, PutOptions{}); err != nil {
		t.Fatal(err)
	}
	s, _ = reload(t, path)
	if _, err := s.Share(root, ShareOptions{}); !errors.Is(err, ErrSharedDiffers) {
		t.Fatalf("different shared settings replaced without confirmation: %v", err)
	}
	result, err := s.Share(root, ShareOptions{Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Registration != "updated" || result.Item.Origin != OriginShared {
		t.Fatalf("share result: %+v", result.Item)
	}
	_, loaded := reload(t, path)
	if origin, _ := loaded.ProjectOrigin(root); origin != OriginShared || loaded.Projects[root].Name != "machine" {
		t.Fatalf("shared project: %s %+v", origin, loaded.Projects[root])
	}
	if local, _ := config.LocalProjectsPath(path); func() bool { b, _ := os.ReadFile(local); return bytes.Contains(b, []byte("machine")) }() {
		t.Fatal("local copy kept after share")
	}
}

// A new project follows projects_storage unless the caller picks a layer.
func TestPutCreateStorage(t *testing.T) {
	for _, tc := range []struct{ setting, storage, want string }{{"", "", OriginShared}, {"machine-local", "", OriginLocal}, {"machine-local", OriginShared, OriginShared}, {"", OriginLocal, OriginLocal}} {
		t.Run(tc.setting+"/"+tc.storage, func(t *testing.T) {
			s, cfg, path, root, _ := serviceFixture(t)
			cfg.ProjectsStorage = tc.setting
			other := filepath.Join(filepath.Dir(root), "other")
			if err := os.MkdirAll(other, 0755); err != nil {
				t.Fatal(err)
			}
			result, err := s.Put(other, config.ManagedProject{Targets: []string{"claude"}, Skills: &config.ResourceTargetConfig{}}, PutOptions{Create: true, Storage: tc.storage})
			if err != nil {
				t.Fatal(err)
			}
			_, loaded := reload(t, path)
			if origin, _ := loaded.ProjectOrigin(other); origin != tc.want || result.Item.Origin != tc.want {
				t.Fatalf("origin %s (result %s), want %s", origin, result.Item.Origin, tc.want)
			}
		})
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
	s, _, path, root, _ := serviceFixture(t)
	release, err := LockConfig(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := s.Localize(root, false); err != nil {
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
