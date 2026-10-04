package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sourcewalk"
)

func TestGetTrackedReposWithFollow(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(ext, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ext, filepath.Join(root, "_repo")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("_repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy, err := GetTrackedRepos(root)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("%v %v", legacy, err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	repos, err := GetTrackedReposWithOptions(root, sourcewalk.Options{Follow: &set})
	if err != nil || len(repos) != 1 || repos[0] != "_repo" {
		t.Fatalf("%v %v", repos, err)
	}
}

// Metadata below a declared entry belongs to the entry's external owner, so it
// is never offered for rehydration, whether the link is present or offline.
func TestMissingTrackedReposSkipFollowedEntries(t *testing.T) {
	remoteURL := makeRemote(t, "")
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "followed"}[linked], func(t *testing.T) {
			root := t.TempDir()
			if linked {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "group")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("group\n"), 0644); err != nil {
				t.Fatal(err)
			}
			store := NewMetadataStore()
			store.Set("group/_repo", &MetadataEntry{Source: remoteURL, Tracked: true})
			store.Set("_plain", &MetadataEntry{Source: remoteURL, Tracked: true})
			if err := store.Save(root); err != nil {
				t.Fatal(err)
			}
			set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
			missing, err := GetMissingTrackedReposWithOptions(root, sourcewalk.Options{Follow: &set})
			if err != nil || len(missing) != 1 || missing[0].Name != "_plain" {
				t.Errorf("missing = %+v, %v; want only _plain", missing, err)
			}
			results, err := RehydrateMissingTrackedRepos(root, ParseOptions{}, InstallOptions{Follow: &set, SkipAudit: true})
			if err != nil || len(results) != 1 || results[0].Name != "_plain" {
				t.Errorf("results = %+v, %v; want only _plain", results, err)
			}
			if _, err := os.Stat(filepath.Join(root, "group", "_repo")); !os.IsNotExist(err) {
				t.Fatalf("rehydrate wrote below the followed entry: %v", err)
			}
			if info, err := os.Lstat(filepath.Join(root, "group")); linked && (err != nil || info.Mode()&os.ModeSymlink == 0) {
				t.Fatalf("group link replaced: %v", err)
			} else if !linked && !os.IsNotExist(err) {
				t.Fatalf("rehydrate created the declared entry: %v", err)
			}
		})
	}
}

// A tracked install never creates a repo inside a declared entry, or the entry
// itself while its link is offline; only an update of an existing repo proceeds.
func TestInstallTrackedRepoRefusesDeclaredEntry(t *testing.T) {
	remoteURL := makeRemote(t, "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("group\n_repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	for _, opts := range []InstallOptions{{Name: "repo"}, {Name: "other", Into: "group"}, {Name: "other", Into: "group", Update: true}} {
		opts.Follow, opts.SkipAudit = &set, true
		if _, err := InstallTrackedRepo(source, root, opts); err == nil || !strings.Contains(err.Error(), "is a link") {
			t.Errorf("%+v: err = %v, want link refusal", opts, err)
		}
	}
	for _, name := range []string{"group", "_repo"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("tracked install created declared entry %s: %v", name, err)
		}
	}
}
